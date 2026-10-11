import io
import json
import os
import queue
import sys
import tempfile
import threading
import types
import unittest
import wave
from pathlib import Path
from unittest.mock import patch

import numpy as np

try:
    import sounddevice  # noqa: F401
except OSError:
    sounddevice_stub = types.ModuleType("sounddevice")
    sounddevice_stub.RawInputStream = None
    sounddevice_stub.OutputStream = None
    sounddevice_stub.CallbackStop = RuntimeError
    sounddevice_stub.query_devices = lambda: []
    sys.modules["sounddevice"] = sounddevice_stub

from voice_sidecar import (
    END_SILENCE_FRAMES,
    FRAME_BYTES,
    FRAME_SAMPLES,
    SAMPLE_RATE,
    VoiceRuntime,
    build_turn_metrics,
    emit,
    frame_is_speech,
    load_simulated_sequence,
    simulated_manifest_path,
    tts_asset_paths,
)


class AlwaysSpeech:
    def is_speech(self, _frame: bytes, _sample_rate: int) -> bool:
        return True


class ProtocolEncodingTest(unittest.TestCase):
    def test_emit_is_safe_on_legacy_windows_stdout(self) -> None:
        buffer = io.BytesIO()
        stdout = io.TextIOWrapper(buffer, encoding="cp1252", errors="strict")
        try:
            with patch("voice_sidecar.sys.stdout", stdout):
                emit("utterance", text="Mèo ơi, dừng lại.")
            stdout.flush()
            raw = buffer.getvalue()
        finally:
            stdout.detach()

        self.assertTrue(raw.isascii())
        self.assertEqual(json.loads(raw)["text"], "Mèo ơi, dừng lại.")

    def test_read_commands_forces_utf8_for_go_pipe(self) -> None:
        raw = (
            json.dumps(
                {"type": "speak", "text": "Mèo ơi, mình ở nhà nhé.", "lang": "vi"},
                ensure_ascii=False,
            ).encode("utf-8")
            + b"\n"
        )
        stdin = io.TextIOWrapper(io.BytesIO(raw), encoding="cp1252", errors="surrogateescape")
        runtime = VoiceRuntime.__new__(VoiceRuntime)
        runtime.command_queue = queue.Queue()
        runtime.stop_event = threading.Event()
        runtime.lifecycle_lock = threading.Lock()
        runtime.playback_cancelled = threading.Event()
        try:
            with patch("voice_sidecar.sys.stdin", stdin):
                runtime.read_commands()
            command = runtime.command_queue.get_nowait()
        finally:
            stdin.detach()

        self.assertEqual(command["text"], "Mèo ơi, mình ở nhà nhé.")


class BoundedJobsTest(unittest.TestCase):
    def test_command_backlog_does_not_block_shutdown_reader(self):
        runtime = VoiceRuntime.__new__(VoiceRuntime)
        runtime.command_queue = queue.Queue(maxsize=1)
        runtime.command_queue.put({"type": "resume"})
        runtime.stop_event = threading.Event()
        runtime.lifecycle_lock = threading.Lock()
        runtime.playback_cancelled = threading.Event()
        finished = threading.Event()

        def read():
            with patch("voice_sidecar.sys.stdin", [json.dumps({"type": "pause"}), json.dumps({"type": "shutdown"})]):
                runtime.read_commands()
            finished.set()

        worker = threading.Thread(target=read, daemon=True)
        worker.start()
        try:
            self.assertTrue(finished.wait(.2), "full queue blocked the shutdown reader")
            self.assertTrue(runtime.stop_event.is_set())
        finally:
            while not finished.is_set():
                try:
                    runtime.command_queue.get(timeout=.2)
                except queue.Empty:
                    break
            worker.join(1)


class WriterBackpressureTest(unittest.TestCase):
    def test_terminal_backlog_is_bounded_and_fails_closed(self):
        from voice_sidecar import JsonlWriter
        entered = threading.Event()
        release = threading.Event()
        retired = threading.Event()

        class BlockedPipe:
            def write(self, text):
                entered.set()
                release.wait(3)
            def flush(self):
                pass

        writer = JsonlWriter()
        writer.max_events = 8
        writer.fatal = retired.set
        try:
            with patch("voice_sidecar.sys.stdout", BlockedPipe()):
                writer.emit("state", state="ready")
                self.assertTrue(entered.wait(1))
                for index in range(100):
                    writer.emit("done", session_id=str(index))
                self.assertTrue(retired.is_set(), "terminal backlog grew without a retirement bound")
                self.assertLessEqual(len(writer.queue), 8)
        finally:
            release.set()
            writer.shutdown()


class LanguageRoutingTest(unittest.TestCase):
    def test_transcribe_uses_auto_language_for_vi_and_ja(self) -> None:
        class Segment:
            text = "ペット、こんにちは"

        class FakeSTT:
            def __init__(self) -> None:
                self.kwargs: dict[str, object] | None = None

            def transcribe(self, _audio, **kwargs):
                self.kwargs = kwargs
                return [Segment()], None

        runtime = VoiceRuntime.__new__(VoiceRuntime)
        runtime.busy = False
        runtime.stt = FakeSTT()
        runtime.pending = {}
        runtime.stop_event = threading.Event()
        runtime.lifecycle_lock = threading.Lock()
        runtime.listen_on_command = False
        with patch("voice_sidecar.emit"):
            runtime.transcribe(bytes(FRAME_BYTES * 2), 123)

        self.assertIsNotNone(runtime.stt.kwargs)
        self.assertNotIn("language", runtime.stt.kwargs)

    def test_japanese_speak_uses_kokoro_and_vietnamese_stays_piper(self) -> None:
        class FakePiperChunk:
            sample_rate = SAMPLE_RATE
            audio_float_array = np.ones(16, dtype=np.float32)

        class FakePiper:
            def __init__(self) -> None:
                self.calls: list[str] = []

            def synthesize(self, text: str):
                self.calls.append(text)
                return [FakePiperChunk()]

        class FakeKokoro:
            def __init__(self) -> None:
                self.calls: list[tuple[str, str]] = []

            def __call__(self, text: str, *, voice: str):
                self.calls.append((text, voice))
                return [(None, None, np.ones(24, dtype=np.float32))]

        runtime = VoiceRuntime.__new__(VoiceRuntime)
        runtime.busy = True
        runtime.pending = {}
        runtime.audio_queue = queue.Queue()
        runtime.listen_enabled = False
        runtime.playback_paused = threading.Event()
        runtime.playback_cancelled = threading.Event()
        runtime.tts = {"vi": FakePiper()}
        runtime.kokoro = FakeKokoro()
        events: list[tuple[str, dict[str, object]]] = []
        with (
            patch("voice_sidecar.play_output", return_value=123),
            patch("voice_sidecar.emit", side_effect=lambda event_type, **payload: events.append((event_type, payload))),
        ):
            runtime.speak("ja-turn", "こんにちは", "ja")
            runtime.speak("vi-turn", "Xin chào", "vi")

        self.assertEqual(runtime.kokoro.calls, [("こんにちは", "jf_alpha")])
        self.assertEqual(runtime.tts["vi"].calls, ["Xin chào"])
        done_langs = [payload["lang"] for event_type, payload in events if event_type == "speak_done"]
        self.assertEqual(done_langs, ["ja", "vi"])

    def test_vi_en_use_distinct_piper_assets(self) -> None:
        vi_model, vi_config = tts_asset_paths("vi")
        en_model, en_config = tts_asset_paths("en")
        self.assertIn("vi_VN-vais1000-medium", vi_model.name)
        self.assertIn("vi_VN-vais1000-medium", vi_config.name)
        self.assertIn("en_US-lessac-medium", en_model.name)
        self.assertIn("en_US-lessac-medium", en_config.name)
        self.assertNotEqual(vi_model, en_model)
        with self.assertRaisesRegex(ValueError, "unsupported speech language"):
            tts_asset_paths("ja")

    def test_direct_speak_routes_language_and_completes_without_stt_metrics(self) -> None:
        class FakeChunk:
            sample_rate = SAMPLE_RATE
            audio_float_array = np.ones(32, dtype=np.float32)

        class FakeVoice:
            def __init__(self) -> None:
                self.calls: list[str] = []

            def synthesize(self, text: str):
                self.calls.append(text)
                return [FakeChunk()]

        runtime = VoiceRuntime.__new__(VoiceRuntime)
        runtime.busy = True
        runtime.pending = {}
        runtime.audio_queue = queue.Queue()
        runtime.listen_enabled = False
        runtime.playback_paused = threading.Event()
        runtime.playback_cancelled = threading.Event()
        runtime.tts = {"vi": FakeVoice(), "en": FakeVoice()}
        events: list[tuple[str, dict[str, object]]] = []
        with (
            patch("voice_sidecar.play_output", return_value=123),
            patch("voice_sidecar.emit", side_effect=lambda event_type, **payload: events.append((event_type, payload))),
            patch("voice_sidecar.time.sleep", return_value=None),
        ):
            runtime.speak("direct-en", "Hello", "en")
            runtime.speak("direct-vi", "Xin chào", "vi")

        self.assertEqual(runtime.tts["vi"].calls, ["Xin chào"])
        self.assertEqual(runtime.tts["en"].calls, ["Hello"])
        done_turns = [payload["turn_id"] for event_type, payload in events if event_type == "speak_done"]
        self.assertEqual(done_turns, ["direct-en", "direct-vi"])
        self.assertFalse(any(event_type == "turn_metrics" for event_type, _ in events))


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
        class FakeWorker:
            def join(self, timeout: float | None = None) -> None:
                return None

            def is_alive(self) -> bool:
                return False

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
            patch.object(runtime, "start_speaker_worker", return_value=FakeWorker()),
            patch.object(runtime, "shutdown"),
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


class ListenOnceTest(unittest.TestCase):
    def test_listen_once_emits_one_utterance_and_returns_idle_without_overlap(self) -> None:
        class Segment:
            text = "open notepad"

        class FakeSTT:
            def transcribe(self, _audio, **_kwargs):
                return [Segment()], None

        runtime = VoiceRuntime.__new__(VoiceRuntime)
        runtime.listen_enabled = True
        runtime.listen_on_command = True
        runtime.listen_once_armed = False
        runtime.busy = True
        runtime.audio_queue = queue.Queue(maxsize=400)
        runtime.command_queue = queue.Queue()
        runtime.stop_event = threading.Event()
        runtime.lifecycle_lock = threading.Lock()
        runtime.pending = {}
        runtime.stt = FakeSTT()

        runtime.command_queue.put({"type": "listen_once"})
        runtime.command_queue.put({"type": "listen_once"})
        events: list[tuple[str, dict[str, object]]] = []
        with patch("voice_sidecar.emit", side_effect=lambda event_type, **payload: events.append((event_type, payload))):
            runtime.handle_commands()
            self.assertFalse(runtime.busy)
            self.assertTrue(runtime.listen_once_armed)
            self.assertEqual([state for event, payload in events if event == "state" for state in [payload.get("state")]], ["listening"])

            runtime.transcribe(bytes(FRAME_BYTES * 2), 123)
            utterances = [payload for event, payload in events if event == "utterance"]
            self.assertEqual(len(utterances), 1)
            self.assertEqual(utterances[0]["text"], "open notepad")
            self.assertTrue(runtime.busy)
            self.assertFalse(runtime.listen_once_armed)
            self.assertEqual(events[-1], ("state", {"state": "idle"}))

            runtime.command_queue.put({"type": "listen_once"})
            runtime.handle_commands()
            self.assertFalse(runtime.busy)
            self.assertTrue(runtime.listen_once_armed)
            listening_states = [payload for event, payload in events if event == "state" and payload.get("state") == "listening"]
            self.assertEqual(len(listening_states), 2)


class ShutdownTranscriptionTest(unittest.TestCase):
    def make_runtime(self) -> VoiceRuntime:
        runtime = VoiceRuntime.__new__(VoiceRuntime)
        runtime.command_queue = queue.Queue()
        runtime.audio_queue = queue.Queue()
        runtime.stop_event = threading.Event()
        runtime.lifecycle_lock = threading.Lock()
        runtime.playback_cancelled = threading.Event()
        runtime.pending = {}
        runtime.busy = False
        runtime.listen_on_command = False
        return runtime

    def test_shutdown_is_latched_before_command_reader_reaches_eof(self) -> None:
        runtime = self.make_runtime()

        def lines():
            yield '{"type":"shutdown"}\n'
            self.assertTrue(runtime.stop_event.is_set())
            self.assertTrue(runtime.playback_cancelled.is_set())

        with patch("voice_sidecar.sys.stdin", lines()):
            runtime.read_commands()
        self.assertTrue(runtime.stop_event.is_set())
        self.assertTrue(runtime.playback_cancelled.is_set())
        self.assertEqual(runtime.command_queue.get_nowait(), {"type": "shutdown"})

    def test_pipe_eof_latches_shutdown_without_waiting_for_control_loop(self) -> None:
        runtime = self.make_runtime()
        with patch("voice_sidecar.sys.stdin", io.StringIO("")):
            runtime.read_commands()
        self.assertTrue(runtime.stop_event.is_set())
        self.assertTrue(runtime.playback_cancelled.is_set())
        self.assertEqual(runtime.command_queue.get_nowait(), {"type": "shutdown"})

    def test_shutdown_before_decode_does_not_call_model_or_emit(self) -> None:
        runtime = self.make_runtime()

        class FakeSTT:
            calls = 0

            def transcribe(self, _audio, **_kwargs):
                self.calls += 1
                return [], None

        runtime.stt = FakeSTT()
        runtime.shutdown()
        with patch("voice_sidecar.emit") as emitted:
            runtime.transcribe(bytes(FRAME_BYTES * 2), 123)
        self.assertEqual(runtime.stt.calls, 0)
        emitted.assert_not_called()
        self.assertTrue(runtime.busy)

    def test_shutdown_during_lazy_decode_does_not_publish_late_utterance(self) -> None:
        runtime = self.make_runtime()

        class FakeSTT:
            def transcribe(self, _audio, **_kwargs):
                def segments():
                    yield types.SimpleNamespace(text="Xin chào")
                    with patch("voice_sidecar.sys.stdin", io.StringIO('{"type":"shutdown"}\n')):
                        runtime.read_commands()
                    yield types.SimpleNamespace(text="open notepad")
                return segments(), None

        runtime.stt = FakeSTT()
        events: list[tuple[str, dict[str, object]]] = []
        with patch("voice_sidecar.emit", side_effect=lambda kind, **payload: events.append((kind, payload))):
            runtime.transcribe(bytes(FRAME_BYTES * 2), 123)
        self.assertTrue(runtime.stop_event.is_set())
        self.assertEqual(events, [("state", {"state": "thinking"})])
        self.assertEqual(runtime.pending, {})
        self.assertTrue(runtime.busy)

    def test_shutdown_does_not_wait_for_active_model_call(self) -> None:
        runtime = self.make_runtime()
        entered = threading.Event()
        release = threading.Event()
        errors: list[BaseException] = []

        class FakeSTT:
            def transcribe(self, _audio, **_kwargs):
                entered.set()
                if not release.wait(timeout=2.0):
                    raise RuntimeError("test model release timed out")
                return [types.SimpleNamespace(text="open notepad")], None

        runtime.stt = FakeSTT()

        def decode():
            try:
                runtime.transcribe(bytes(FRAME_BYTES * 2), 123)
            except BaseException as exc:
                errors.append(exc)

        events: list[tuple[str, dict[str, object]]] = []
        with patch("voice_sidecar.emit", side_effect=lambda kind, **payload: events.append((kind, payload))):
            worker = threading.Thread(target=decode)
            worker.start()
            try:
                self.assertTrue(entered.wait(timeout=1.0))
                runtime.shutdown()
                self.assertTrue(runtime.stop_event.is_set())
                self.assertTrue(worker.is_alive())
            finally:
                release.set()
                worker.join(timeout=2.0)
        self.assertFalse(worker.is_alive())
        self.assertEqual(errors, [])
        self.assertEqual(events, [("state", {"state": "thinking"})])
        self.assertEqual(runtime.pending, {})

    def test_shutdown_before_publication_discards_completed_result(self) -> None:
        runtime = self.make_runtime()

        class FakeSTT:
            def transcribe(self, _audio, **_kwargs):
                return [types.SimpleNamespace(text="open notepad")], None

        runtime.stt = FakeSTT()
        events: list[tuple[str, dict[str, object]]] = []

        def completed_at():
            runtime.shutdown()
            return 456

        with (
            patch("voice_sidecar.time.perf_counter_ns", side_effect=completed_at),
            patch("voice_sidecar.emit", side_effect=lambda kind, **payload: events.append((kind, payload))),
        ):
            runtime.transcribe(bytes(FRAME_BYTES * 2), 123)
        self.assertEqual(events, [("state", {"state": "thinking"})])
        self.assertEqual(runtime.pending, {})

    def test_error_after_pipe_eof_does_not_publish_or_resume(self) -> None:
        runtime = self.make_runtime()

        class FakeSTT:
            def transcribe(self, _audio, **_kwargs):
                with patch("voice_sidecar.sys.stdin", io.StringIO("")):
                    runtime.read_commands()
                raise RuntimeError("decode interrupted")

        runtime.stt = FakeSTT()
        events: list[tuple[str, dict[str, object]]] = []
        with patch("voice_sidecar.emit", side_effect=lambda kind, **payload: events.append((kind, payload))):
            runtime.transcribe(bytes(FRAME_BYTES * 2), 123)
        self.assertEqual(events, [("state", {"state": "thinking"})])
        self.assertEqual(runtime.pending, {})
        self.assertTrue(runtime.busy)


class DictationIntegrationTest(unittest.TestCase):
    def make_runtime(self):
        runtime = VoiceRuntime.__new__(VoiceRuntime)
        runtime.command_queue = queue.Queue()
        runtime.speaker_queue = queue.Queue()
        runtime.stop_event = threading.Event()
        runtime.lifecycle_lock = threading.Lock()
        runtime.playback_cancelled = threading.Event()
        runtime.playback_active = threading.Event()
        runtime.listen_enabled = False
        runtime.dictation = types.SimpleNamespace(
            active=False, start=lambda sid: True, stop=lambda sid: True,
            cancel=lambda sid: True, shutdown=lambda: None,
        )
        return runtime

    def test_reader_latches_matching_stop_cancel_without_control_loop(self):
        runtime = self.make_runtime()
        from unittest.mock import Mock
        runtime.dictation.stop = Mock(return_value=True)
        runtime.dictation.cancel = Mock(return_value=True)
        runtime.dictation.shutdown = Mock()
        lines = [
            '{"type":"dictation_stop","protocol_version":1,"session_id":"owned"}\n',
            '{"type":"dictation_cancel","protocol_version":1,"session_id":"owned"}\n',
        ]
        with patch("voice_sidecar.sys.stdin", lines):
            runtime.read_commands()
        runtime.dictation.stop.assert_called_once_with("owned")
        runtime.dictation.cancel.assert_called_once_with("owned")
        runtime.dictation.shutdown.assert_called_once()
        self.assertEqual(runtime.command_queue.get_nowait(), {"type": "shutdown"})

    def test_reader_preserves_start_then_immediate_stop_order(self):
        runtime = self.make_runtime()
        calls = []
        runtime.dictation.start = lambda sid: calls.append(("start", sid)) or True
        runtime.dictation.stop = lambda sid: calls.append(("stop", sid)) or True
        with patch("voice_sidecar.sys.stdin", [
            '{"type":"dictation_start","protocol_version":1,"session_id":"quick"}\n',
            '{"type":"dictation_stop","protocol_version":1,"session_id":"quick"}\n',
        ]):
            runtime.read_commands()
        self.assertEqual(calls, [("start", "quick"), ("stop", "quick")])

    def test_dictation_start_is_separate_from_legacy_and_rejects_protocol_or_busy(self):
        runtime = self.make_runtime()
        from unittest.mock import Mock
        runtime.dictation.start = Mock(return_value=True)
        valid = {"type": "dictation_start", "protocol_version": 1, "session_id": "owned"}
        with patch("voice_sidecar.emit"):
            runtime.command_queue.put({**valid, "protocol_version": 2})
            runtime.handle_commands()
            runtime.dictation.start.assert_not_called()
            runtime.listen_enabled = True
            runtime.command_queue.put(valid)
            runtime.handle_commands()
            runtime.dictation.start.assert_not_called()
            runtime.listen_enabled = False
            runtime.playback_active.set()
            runtime.command_queue.put(valid)
            runtime.handle_commands()
            runtime.dictation.start.assert_not_called()
            runtime.playback_active.clear()
            runtime.command_queue.put(valid)
            runtime.handle_commands()
        runtime.dictation.start.assert_called_once_with("owned")
        self.assertTrue(runtime.speaker_queue.empty())

    def test_legacy_speak_cannot_take_dictation_model_or_audio_ownership(self):
        runtime = self.make_runtime()
        runtime.dictation.active = True
        runtime.command_queue.put({"type": "speak", "text": "must not speak"})
        with patch("voice_sidecar.emit"):
            runtime.handle_commands()
        self.assertTrue(runtime.speaker_queue.empty())


class PlaybackControlTest(unittest.TestCase):
    def make_runtime(self) -> VoiceRuntime:
        runtime = VoiceRuntime.__new__(VoiceRuntime)
        runtime.stop_event = threading.Event()
        runtime.lifecycle_lock = threading.Lock()
        runtime.speaker_queue = queue.Queue()
        runtime.playback_paused = threading.Event()
        runtime.playback_cancelled = threading.Event()
        runtime.playback_active = threading.Event()
        runtime.speaker_thread = None
        return runtime

    def test_pause_resume_are_idempotent(self) -> None:
        runtime = self.make_runtime()
        runtime.pause_playback()
        runtime.pause_playback()
        self.assertTrue(runtime.playback_paused.is_set())
        runtime.resume_playback()
        runtime.resume_playback()
        self.assertFalse(runtime.playback_paused.is_set())

    def test_skip_and_stop_cancel_active_playback(self) -> None:
        runtime = self.make_runtime()
        runtime.playback_active.set()
        runtime.skip_playback()
        self.assertTrue(runtime.playback_cancelled.is_set())
        runtime.playback_cancelled.clear()
        runtime.stop_playback()
        runtime.stop_playback()
        self.assertTrue(runtime.playback_cancelled.is_set())

    def test_single_speaker_worker_is_reused_and_shutdown_joins_it(self) -> None:
        runtime = self.make_runtime()
        worker = runtime.start_speaker_worker()
        self.assertIs(worker, runtime.start_speaker_worker())
        runtime.shutdown()
        worker.join(timeout=1.0)
        self.assertFalse(worker.is_alive())


if __name__ == "__main__":
    unittest.main()
