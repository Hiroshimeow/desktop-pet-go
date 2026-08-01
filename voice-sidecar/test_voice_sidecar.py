import json
import os
import queue
import tempfile
import threading
import unittest
import wave
from pathlib import Path
from unittest.mock import patch

import numpy as np

from voice_sidecar import (
    END_SILENCE_FRAMES,
    FRAME_BYTES,
    FRAME_SAMPLES,
    SAMPLE_RATE,
    VoiceRuntime,
    build_turn_metrics,
    frame_is_speech,
    load_simulated_sequence,
    simulated_manifest_path,
)


class AlwaysSpeech:
    def is_speech(self, _frame: bytes, _sample_rate: int) -> bool:
        return True


class VoiceMetricTest(unittest.TestCase):
    def test_uses_nonzero_first_audio_field_for_latency(self) -> None:
        metrics = build_turn_metrics(
            "turn-1",
            {"eos_ns": 1_000_000_000, "stt_done_ns": 1_200_000_000},
            tts_done_ns=1_500_000_000,
            first_audio_ns=1_750_000_000,
        )
        self.assertEqual(metrics["first_audio_ns"], 1_750_000_000)
        self.assertEqual(metrics["latency_ms"], 750.0)
        self.assertIn('"first_audio_ns": 1750000000', json.dumps(metrics))
        self.assertNotIn("audio_start_ns", metrics)

    def test_resume_discards_pending_turns(self) -> None:
        runtime = VoiceRuntime.__new__(VoiceRuntime)
        runtime.audio_queue = queue.Queue()
        runtime.pending = {}
        runtime.busy = True
        with patch("voice_sidecar.emit"):
            for index in range(100):
                turn_id = f"ignored-{index}"
                runtime.pending[turn_id] = {"eos_ns": index, "stt_done_ns": index}
                runtime.resume_listening(turn_id)
        self.assertEqual(runtime.pending, {})


class SimulatedInputTest(unittest.TestCase):
    def write_wav(self, path: Path, *, sample_rate: int = SAMPLE_RATE) -> None:
        samples = np.full(FRAME_SAMPLES * 2, 1024, dtype=np.int16)
        with wave.open(str(path), "wb") as output:
            output.setnchannels(1)
            output.setsampwidth(2)
            output.setframerate(sample_rate)
            output.writeframes(samples.tobytes())

    def test_simulated_pcm_reaches_existing_audio_queue(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            self.write_wav(root / "utterance.wav")
            manifest = root / "sequence.json"
            manifest.write_text(
                json.dumps({"items": [{"wav": "utterance.wav", "delay_ms": 0}]}),
                encoding="utf-8",
            )
            sequence = load_simulated_sequence(manifest)

        runtime = VoiceRuntime.__new__(VoiceRuntime)
        runtime.audio_queue = queue.Queue(maxsize=400)
        runtime.stop_event = threading.Event()
        runtime.busy = False
        with patch("voice_sidecar.time.sleep", return_value=None):
            runtime.feed_simulated_sequence(sequence)

        queued = [runtime.audio_queue.get_nowait()[1] for _ in range(runtime.audio_queue.qsize())]
        self.assertEqual(len(queued), 2 + END_SILENCE_FRAMES)
        self.assertEqual({len(frame) for frame in queued}, {FRAME_BYTES})
        self.assertTrue(frame_is_speech(queued[0], AlwaysSpeech()))
        self.assertEqual(queued[-END_SILENCE_FRAMES:], [bytes(FRAME_BYTES)] * END_SILENCE_FRAMES)

    def test_simulated_source_is_disabled_by_default(self) -> None:
        self.assertIsNone(simulated_manifest_path({}))

    def test_default_run_still_opens_raw_microphone(self) -> None:
        runtime = VoiceRuntime.__new__(VoiceRuntime)
        runtime.audio_queue = queue.Queue(maxsize=400)
        runtime.command_queue = queue.Queue()
        runtime.stop_event = threading.Event()
        runtime.pending = {}
        runtime.busy = True
        with (
            patch.dict(os.environ, {}, clear=True),
            patch("voice_sidecar.sd.RawInputStream") as raw_input,
            patch("voice_sidecar.emit"),
            patch("voice_sidecar.threading.Thread.start"),
            patch.object(runtime, "listen_loop"),
        ):
            runtime.run()
        raw_input.assert_called_once_with(
            samplerate=SAMPLE_RATE,
            blocksize=FRAME_SAMPLES,
            channels=1,
            dtype="int16",
            callback=runtime.input_callback,
        )

    def test_rejects_non_16khz_pcm_wav(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            self.write_wav(root / "bad.wav", sample_rate=8_000)
            manifest = root / "sequence.json"
            manifest.write_text(
                json.dumps({"items": [{"wav": "bad.wav"}]}),
                encoding="utf-8",
            )
            with self.assertRaisesRegex(RuntimeError, "16000 Hz, 16-bit mono PCM"):
                load_simulated_sequence(manifest)


class FrameSpeechGateTest(unittest.TestCase):
    def test_rejects_digital_floor_even_if_vad_fires(self) -> None:
        floor = np.ones(480, dtype=np.int16).tobytes()
        self.assertFalse(frame_is_speech(floor, AlwaysSpeech()))

    def test_allows_audible_frame_to_reach_vad(self) -> None:
        audible = np.full(480, 512, dtype=np.int16).tobytes()
        self.assertTrue(frame_is_speech(audible, AlwaysSpeech()))


if __name__ == "__main__":
    unittest.main()
