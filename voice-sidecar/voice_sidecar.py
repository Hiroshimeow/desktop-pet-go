from __future__ import annotations

import argparse
import hashlib
import json
import os
import queue
import sys
import threading
import time
import urllib.request
import uuid
import wave
from collections import deque
from collections.abc import Mapping
from pathlib import Path
from typing import Any

import numpy as np
import sounddevice as sd
import webrtcvad
from faster_whisper import WhisperModel
from huggingface_hub import snapshot_download
from piper import PiperVoice

SAMPLE_RATE = 16_000
FRAME_MS = 30
FRAME_SAMPLES = SAMPLE_RATE * FRAME_MS // 1000
FRAME_BYTES = FRAME_SAMPLES * 2
FRAME_NS = FRAME_MS * 1_000_000
END_SILENCE_FRAMES = 12
MIN_SPEECH_FRAMES = 7
MAX_UTTERANCE_FRAMES = 8_000 // FRAME_MS
COOLDOWN_SECONDS = 0.4
MIN_FRAME_PEAK = 64
SIMULATED_WAV_SEQUENCE_ENV = "DESKTOP_PET_VOICE_TEST_WAV_SEQUENCE"

STT_REPO = "Systran/faster-whisper-base"
STT_REVISION = "ebe41f70d5b6dfa9166e2c581c45c9c0cfc57b66"
VOICE_NAME = "vi_VN-vais1000-medium"
VOICE_BASE_URL = "https://huggingface.co/rhasspy/piper-voices/resolve/main/vi/vi_VN/vais1000/medium"
EN_VOICE_NAME = "en_US-lessac-medium"
EN_VOICE_BASE_URL = "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium"
KOKORO_VOICE = "jf_alpha"
KOKORO_SAMPLE_RATE = 24_000

REPO_ROOT = Path(__file__).resolve().parents[1]
ASSET_ROOT = REPO_ROOT / ".voice"
STT_DIR = ASSET_ROOT / "models" / "faster-whisper-base"
TTS_DIR = ASSET_ROOT / "models" / "piper"
LOG_DIR = ASSET_ROOT / "logs"
TTS_MODEL = TTS_DIR / f"{VOICE_NAME}.onnx"
TTS_CONFIG = TTS_DIR / f"{VOICE_NAME}.onnx.json"
EN_TTS_MODEL = TTS_DIR / f"{EN_VOICE_NAME}.onnx"
EN_TTS_CONFIG = TTS_DIR / f"{EN_VOICE_NAME}.onnx.json"

EXPECTED_SHA256 = {
    STT_DIR / "config.json": "56a6d8110d311f19c8f0471e562832c7527f146b567275bfca59fcf7c184da9a",
    STT_DIR / "model.bin": "d01c3014881c9c6f3133c182f3d2887eb6ca1c789a7538c5c007196857a0a6a9",
    STT_DIR / "tokenizer.json": "fb7b63191e9bb045082c79fd742a3106a12c99513ab30df4a0d47fa6cb6fd0ab",
    STT_DIR / "vocabulary.txt": "34ce3fe1c5041027b3f8d42912270993f986dbc4bb34cf27f951e34a1e453913",
    TTS_MODEL: "ec7c89e2c85f4d1edc24b6120c18aaf1bda614f06b511567eb9c7c0de15e2dab",
    TTS_CONFIG: "fafb9da1354ed4b77c31af228ed41fb41cd825c14cffa105454b25e6ae751ee0",
    EN_TTS_MODEL: "5efe09e69902187827af646e1a6e9d269dee769f9877d17b16b1b46eeaaf019f",
    EN_TTS_CONFIG: "efe19c417bed055f2d69908248c6ba650fa135bc868b0e6abb3da181dab690a0",
}


def tts_asset_paths(lang: str) -> tuple[Path, Path]:
    if lang == "vi":
        return TTS_MODEL, TTS_CONFIG
    if lang == "en":
        return EN_TTS_MODEL, EN_TTS_CONFIG
    raise ValueError(f"unsupported speech language {lang!r}; use 'vi' or 'en'")


def load_japanese_kokoro() -> Any:
    from kokoro import KPipeline

    return KPipeline(lang_code="j")


def synthesize_japanese_kokoro(pipeline: Any, text: str) -> np.ndarray:
    audio_chunks = [np.asarray(audio, dtype=np.float32).reshape(-1) for _, _, audio in pipeline(text, voice=KOKORO_VOICE)]
    audio_chunks = [audio for audio in audio_chunks if audio.size]
    if not audio_chunks:
        raise RuntimeError("Kokoro returned no Japanese audio")
    return np.concatenate(audio_chunks)


def emit(event_type: str, **payload: Any) -> None:
    print(json.dumps({"type": event_type, **payload}, ensure_ascii=True), flush=True)


def diagnostic(message: str) -> None:
    print(message, file=sys.stderr, flush=True)


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def frame_is_speech(frame: bytes, vad: Any) -> bool:
    samples = np.frombuffer(frame, dtype=np.int16)
    if samples.size == 0:
        return False
    peak = int(np.max(np.abs(samples.astype(np.int32))))
    return peak >= MIN_FRAME_PEAK and bool(vad.is_speech(frame, SAMPLE_RATE))


def simulated_manifest_path(environ: Mapping[str, str] | None = None) -> Path | None:
    source = os.environ if environ is None else environ
    value = source.get(SIMULATED_WAV_SEQUENCE_ENV, "").strip()
    return Path(value).expanduser().resolve() if value else None


def load_simulated_sequence(manifest_path: Path) -> list[tuple[float, list[bytes]]]:
    manifest_path = Path(manifest_path).expanduser().resolve()
    try:
        payload = json.loads(manifest_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise RuntimeError(f"cannot read simulated voice manifest {manifest_path}: {exc}") from exc
    items = payload.get("items") if isinstance(payload, dict) else None
    if not isinstance(items, list) or not items:
        raise RuntimeError(f"simulated voice manifest must contain a non-empty items list: {manifest_path}")

    sequence: list[tuple[float, list[bytes]]] = []
    for index, item in enumerate(items):
        if not isinstance(item, dict) or not isinstance(item.get("wav"), str) or not item["wav"].strip():
            raise RuntimeError(f"simulated voice item {index} must contain a wav path")
        delay_ms = item.get("delay_ms", 0)
        if not isinstance(delay_ms, (int, float)) or delay_ms < 0:
            raise RuntimeError(f"simulated voice item {index} delay_ms must be non-negative")
        wav_path = (manifest_path.parent / item["wav"]).resolve()
        try:
            with wave.open(str(wav_path), "rb") as input_wav:
                valid = (
                    input_wav.getnchannels() == 1
                    and input_wav.getsampwidth() == 2
                    and input_wav.getframerate() == SAMPLE_RATE
                    and input_wav.getcomptype() == "NONE"
                )
                pcm = input_wav.readframes(input_wav.getnframes()) if valid else b""
        except (OSError, wave.Error) as exc:
            raise RuntimeError(f"cannot read simulated WAV {wav_path}: {exc}") from exc
        if not valid:
            raise RuntimeError(f"simulated WAV must be 16000 Hz, 16-bit mono PCM: {wav_path}")
        if not pcm:
            raise RuntimeError(f"simulated WAV contains no audio: {wav_path}")
        frames = [pcm[offset : offset + FRAME_BYTES].ljust(FRAME_BYTES, b"\0") for offset in range(0, len(pcm), FRAME_BYTES)]
        frames.extend([bytes(FRAME_BYTES)] * END_SILENCE_FRAMES)
        sequence.append((float(delay_ms) / 1000.0, frames))
    return sequence


def build_turn_metrics(
    turn_id: str,
    timing: dict[str, int],
    *,
    tts_done_ns: int,
    first_audio_ns: int,
) -> dict[str, int | float | str]:
    eos_ns = int(timing.get("eos_ns", 0))
    if eos_ns <= 0 or first_audio_ns <= eos_ns:
        raise RuntimeError("invalid first-audio timing")
    return {
        "turn_id": turn_id,
        "eos_ns": eos_ns,
        "stt_done_ns": int(timing.get("stt_done_ns", 0)),
        "tts_done_ns": tts_done_ns,
        "first_audio_ns": first_audio_ns,
        "latency_ms": (first_audio_ns - eos_ns) / 1_000_000,
    }


def play_output(
    audio: np.ndarray,
    sample_rate: int,
    *,
    paused: threading.Event | None = None,
    cancelled: threading.Event | None = None,
) -> int:
    samples = np.asarray(audio, dtype=np.float32).reshape(-1)
    if samples.size == 0:
        raise RuntimeError("Piper returned empty audio")

    paused = paused or threading.Event()
    cancelled = cancelled or threading.Event()
    first_audio_ready = threading.Event()
    finished = threading.Event()
    state: dict[str, Any] = {"offset": 0, "first_audio_ns": 0, "status": None}
    stream: Any = None

    def callback(outdata: Any, frames: int, time_info: Any, status: Any) -> None:
        if status and state["status"] is None:
            state["status"] = str(status)
        if state["first_audio_ns"] == 0:
            first_audio_ns = time.perf_counter_ns()
            try:
                dac_time = float(time_info.outputBufferDacTime)
                stream_time = float(stream.time)
                if dac_time > stream_time:
                    first_audio_ns += int((dac_time - stream_time) * 1_000_000_000)
            except (AttributeError, TypeError, ValueError):
                pass
            state["first_audio_ns"] = first_audio_ns
            first_audio_ready.set()

        outdata.fill(0)
        if cancelled.is_set():
            raise sd.CallbackStop
        if paused.is_set():
            return

        offset = int(state["offset"])
        count = min(frames, samples.size - offset)
        if count > 0:
            outdata[:count, 0] = samples[offset : offset + count]
            state["offset"] = offset + count
        if int(state["offset"]) >= samples.size:
            raise sd.CallbackStop

    timeout = max(5.0, samples.size / sample_rate + 5.0)
    stream = sd.OutputStream(
        samplerate=sample_rate,
        channels=1,
        dtype="float32",
        callback=callback,
        finished_callback=finished.set,
    )
    with stream:
        if not first_audio_ready.wait(timeout=2.0):
            raise RuntimeError("speaker produced no first output callback")
        deadline = time.monotonic() + timeout
        while not finished.wait(timeout=0.05):
            if paused.is_set():
                deadline = time.monotonic() + timeout
            elif time.monotonic() >= deadline:
                raise RuntimeError("speaker playback timed out")
    if state["status"]:
        diagnostic(f"speaker status: {state['status']}")
    if cancelled.is_set():
        return 0
    return int(state["first_audio_ns"])


def verify_assets() -> None:
    for path, expected in EXPECTED_SHA256.items():
        if not path.is_file():
            raise RuntimeError(f"missing voice asset: {path}; run scripts\\setup-voice.ps1")
        actual = sha256(path)
        if actual != expected:
            raise RuntimeError(f"checksum mismatch for {path}: {actual} != {expected}")


def download_file(url: str, path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temp = path.with_suffix(path.suffix + ".part")
    with urllib.request.urlopen(url, timeout=120) as response, temp.open("wb") as output:
        while chunk := response.read(1024 * 1024):
            output.write(chunk)
    temp.replace(path)


def setup_assets() -> None:
    ASSET_ROOT.mkdir(parents=True, exist_ok=True)
    diagnostic(f"downloading STT {STT_REPO}@{STT_REVISION}")
    snapshot_download(
        repo_id=STT_REPO,
        revision=STT_REVISION,
        local_dir=STT_DIR,
        allow_patterns=["config.json", "model.bin", "tokenizer.json", "vocabulary.txt"],
    )
    for voice_name, base_url in (
        (VOICE_NAME, VOICE_BASE_URL),
        (EN_VOICE_NAME, EN_VOICE_BASE_URL),
    ):
        for suffix in (".onnx", ".onnx.json"):
            path = TTS_DIR / f"{voice_name}{suffix}"
            if not path.exists() or sha256(path) != EXPECTED_SHA256[path]:
                diagnostic(f"downloading TTS {voice_name}{suffix}")
                download_file(f"{base_url}/{voice_name}{suffix}?download=true", path)
    verify_assets()
    stt = WhisperModel(
        str(STT_DIR),
        device="cpu",
        compute_type="int8",
        cpu_threads=0,
        local_files_only=True,
    )
    for lang, sample in (("vi", "Xin chào, kiểm tra giọng nói."), ("en", "Hello, voice check.")):
        model, config = tts_asset_paths(lang)
        voice = PiperVoice.load(model, config)
        chunks = list(voice.synthesize(sample))
        if not chunks or sum(len(chunk.audio_float_array) for chunk in chunks) == 0:
            raise RuntimeError(f"Piper {lang} self-check produced no audio")
    kokoro = load_japanese_kokoro()
    if synthesize_japanese_kokoro(kokoro, "こんにちは。音声を確認します。").size == 0:
        raise RuntimeError("Kokoro Japanese self-check produced no audio")
    total = sum(path.stat().st_size for path in EXPECTED_SHA256)
    print(
        f"voice setup OK: {total / 1024 / 1024:.1f} MiB, STT={STT_REPO}@{STT_REVISION}, "
        f"TTS={VOICE_NAME},{EN_VOICE_NAME},Kokoro-82M:{KOKORO_VOICE}"
    )
    del stt


class VoiceRuntime:
    def __init__(self, *, listen_enabled: bool = True, listen_on_command: bool = False) -> None:
        verify_assets()
        self.stt = WhisperModel(
            str(STT_DIR),
            device="cpu",
            compute_type="int8",
            cpu_threads=0,
            local_files_only=True,
        )
        self.tts: dict[str, Any] = {"vi": PiperVoice.load(TTS_MODEL, TTS_CONFIG)}
        self.kokoro: Any = None
        self.listen_enabled = listen_enabled
        self.listen_on_command = listen_on_command
        self.listen_once_armed = False
        self.vad = webrtcvad.Vad(2)
        self.audio_queue: queue.Queue[tuple[int, bytes]] = queue.Queue(maxsize=400)
        self.command_queue: queue.Queue[dict[str, Any]] = queue.Queue()
        self.speaker_queue: queue.Queue[dict[str, Any]] = queue.Queue()
        self.stop_event = threading.Event()
        self.playback_paused = threading.Event()
        self.playback_cancelled = threading.Event()
        self.playback_active = threading.Event()
        self.speaker_thread: threading.Thread | None = None
        self.busy = True
        self.pending: dict[str, dict[str, int]] = {}
        LOG_DIR.mkdir(parents=True, exist_ok=True)
        self.metrics_path = LOG_DIR / "turns.jsonl"

    def enqueue_frame(self, frame: bytes, at_ns: int | None = None) -> bool:
        if self.busy or len(frame) != FRAME_BYTES:
            return False
        timestamp = time.perf_counter_ns() if at_ns is None else at_ns
        try:
            self.audio_queue.put_nowait((timestamp, frame))
        except queue.Full:
            try:
                self.audio_queue.get_nowait()
                self.audio_queue.put_nowait((timestamp, frame))
            except queue.Empty:
                return False
        return True

    def input_callback(self, indata: Any, frames: int, _time_info: Any, status: Any) -> None:
        if status:
            diagnostic(f"microphone status: {status}")
        if frames == FRAME_SAMPLES:
            self.enqueue_frame(bytes(indata))

    def feed_simulated_sequence(self, sequence: list[tuple[float, list[bytes]]]) -> None:
        for index, (delay_seconds, frames) in enumerate(sequence):
            while self.busy:
                if self.stop_event.is_set():
                    return
                time.sleep(0.01)
            if self.stop_event.wait(delay_seconds):
                return
            for frame in frames:
                while self.busy:
                    if self.stop_event.is_set():
                        return
                    time.sleep(0.01)
                self.enqueue_frame(frame)
                if self.stop_event.wait(FRAME_MS / 1000.0):
                    return
            if index + 1 == len(sequence):
                continue
            deadline = time.monotonic() + 15.0
            while not self.busy:
                if self.stop_event.is_set():
                    return
                if time.monotonic() >= deadline:
                    raise RuntimeError("simulated WAV did not reach the VAD/STT endpoint")
                time.sleep(0.005)

    def run_simulated_sequence(self, sequence: list[tuple[float, list[bytes]]]) -> None:
        try:
            self.feed_simulated_sequence(sequence)
        except Exception as exc:
            diagnostic(f"simulated voice input failed: {exc}")
            emit("state", state="error", detail=f"simulated voice input failed: {exc}")
            self.stop_event.set()

    def read_commands(self) -> None:
        if hasattr(sys.stdin, "reconfigure"):
            sys.stdin.reconfigure(encoding="utf-8", errors="strict")
        for line in sys.stdin:
            try:
                command = json.loads(line)
                if isinstance(command, dict):
                    self.command_queue.put(command)
            except json.JSONDecodeError as exc:
                diagnostic(f"invalid command JSON: {exc}")
        self.command_queue.put({"type": "shutdown"})

    def clear_audio(self) -> None:
        while True:
            try:
                self.audio_queue.get_nowait()
            except queue.Empty:
                return

    def resume_listening(self, turn_id: str = "") -> None:
        if turn_id:
            self.pending.pop(turn_id, None)
        self.clear_audio()
        stop_event = getattr(self, "stop_event", None)
        if stop_event is not None and stop_event.is_set():
            self.busy = True
            return
        if not getattr(self, "listen_enabled", True):
            self.busy = True
            emit("state", state="idle")
            return
        if getattr(self, "listen_on_command", False):
            self.listen_once_armed = False
            self.busy = True
            emit("state", state="idle")
            return
        self.busy = False
        emit("state", state="listening")

    def arm_listen_once(self) -> None:
        if not getattr(self, "listen_enabled", True) or not getattr(self, "listen_on_command", False):
            return
        if getattr(self, "listen_once_armed", False) or not self.busy:
            return
        self.clear_audio()
        self.listen_once_armed = True
        self.busy = False
        emit("state", state="listening")

    def pause_playback(self) -> None:
        self.playback_paused.set()

    def resume_playback(self) -> None:
        self.playback_paused.clear()

    def skip_playback(self) -> None:
        self.playback_cancelled.set()

    def stop_playback(self) -> None:
        self.playback_cancelled.set()

    def shutdown(self) -> None:
        self.playback_cancelled.set()
        self.stop_event.set()

    def start_speaker_worker(self) -> threading.Thread:
        worker = self.speaker_thread
        if worker is not None and worker.is_alive():
            return worker
        worker = threading.Thread(target=self.speaker_loop, name="voice-speaker", daemon=True)
        self.speaker_thread = worker
        worker.start()
        return worker

    def speaker_loop(self) -> None:
        while not self.stop_event.is_set():
            try:
                command = self.speaker_queue.get(timeout=0.03)
            except queue.Empty:
                continue
            if self.stop_event.is_set():
                break
            self.playback_active.set()
            try:
                self.speak(
                    str(command.get("turn_id", "")),
                    str(command.get("text", "")),
                    str(command.get("lang", "vi")),
                )
            finally:
                self.playback_active.clear()
            if self.stop_event.wait(COOLDOWN_SECONDS):
                break
            if self.speaker_queue.empty():
                self.resume_listening()

    def tts_for_language(self, lang: str) -> Any:
        model, config = tts_asset_paths(lang)
        voice = self.tts.get(lang)
        if voice is None:
            voice = PiperVoice.load(model, config)
            self.tts[lang] = voice
        return voice

    def transcribe(self, pcm: bytes, eos_ns: int) -> None:
        self.busy = True
        emit("state", state="thinking")
        turn_id = uuid.uuid4().hex[:12]
        try:
            audio = np.frombuffer(pcm, dtype=np.int16).astype(np.float32) / 32768.0
            segments, _ = self.stt.transcribe(
                audio,
                beam_size=1,
                best_of=1,
                temperature=0,
                condition_on_previous_text=False,
                vad_filter=False,
            )
            text = " ".join(segment.text.strip() for segment in segments).strip()
            stt_done_ns = time.perf_counter_ns()
            if not text:
                diagnostic("empty transcription; returning to wake listening")
                self.resume_listening()
                return
            if getattr(self, "listen_on_command", False):
                emit("utterance", turn_id=turn_id, text=text, eos_ns=eos_ns, stt_done_ns=stt_done_ns)
                self.resume_listening(turn_id)
                return
            self.pending[turn_id] = {"eos_ns": eos_ns, "stt_done_ns": stt_done_ns}
            emit("utterance", turn_id=turn_id, text=text, eos_ns=eos_ns, stt_done_ns=stt_done_ns)
        except Exception as exc:
            diagnostic(f"STT failed: {exc}")
            emit("state", state="error", detail=f"STT failed: {exc}")
            self.resume_listening()

    def speak(self, turn_id: str, text: str, lang: str = "vi") -> None:
        self.busy = True
        emit("state", state="speaking", turn_id=turn_id, lang=lang)
        timing = self.pending.pop(turn_id, None)
        try:
            if lang == "ja":
                if getattr(self, "kokoro", None) is None:
                    self.kokoro = load_japanese_kokoro()
                audio = synthesize_japanese_kokoro(self.kokoro, text)
                sample_rate = KOKORO_SAMPLE_RATE
            else:
                voice = self.tts_for_language(lang)
                chunks = list(voice.synthesize(text))
                if not chunks:
                    raise RuntimeError("Piper returned no audio")
                sample_rate = chunks[0].sample_rate
                audio = np.concatenate([chunk.audio_float_array for chunk in chunks])
            tts_done_ns = time.perf_counter_ns()
            first_audio_ns = play_output(
                audio,
                sample_rate,
                paused=self.playback_paused,
                cancelled=self.playback_cancelled,
            )
            if first_audio_ns:
                emit("first_audio", turn_id=turn_id, first_audio_ns=first_audio_ns)
                if timing:
                    metrics = build_turn_metrics(
                        turn_id,
                        timing,
                        tts_done_ns=tts_done_ns,
                        first_audio_ns=first_audio_ns,
                    )
                    with self.metrics_path.open("a", encoding="utf-8") as output:
                        output.write(json.dumps(metrics, ensure_ascii=False) + "\n")
                    emit("turn_metrics", **metrics)
        except Exception as exc:
            diagnostic(f"TTS/playback failed: {exc}")
            emit("state", state="error", detail=f"TTS/playback failed: {exc}")
        finally:
            cancelled = self.playback_cancelled.is_set()
            self.playback_paused.clear()
            self.playback_cancelled.clear()
            emit("speak_done", turn_id=turn_id, lang=lang, cancelled=cancelled)

    def handle_commands(self) -> None:
        while True:
            try:
                command = self.command_queue.get_nowait()
            except queue.Empty:
                return
            command_type = command.get("type")
            if command_type == "speak":
                self.busy = True
                self.speaker_queue.put(command)
            elif command_type == "listen_once":
                self.arm_listen_once()
            elif command_type == "resume":
                self.resume_listening(str(command.get("turn_id", "")))
            elif command_type == "pause":
                self.pause_playback()
            elif command_type == "resume_playback":
                self.resume_playback()
            elif command_type == "skip":
                self.skip_playback()
            elif command_type == "stop_playback":
                self.stop_playback()
            elif command_type == "shutdown":
                self.shutdown()
                return
            else:
                diagnostic(f"unknown command: {command_type!r}")

    def listen_loop(self) -> None:
        pre_roll: deque[bytes] = deque(maxlen=10)
        utterance: list[bytes] = []
        speech_frames = 0
        silence_frames = 0
        last_voice_ns = 0
        while not self.stop_event.is_set():
            self.handle_commands()
            if self.stop_event.is_set():
                break
            try:
                at_ns, frame = self.audio_queue.get(timeout=0.03)
            except queue.Empty:
                continue
            voiced = frame_is_speech(frame, self.vad)
            if not utterance:
                pre_roll.append(frame)
                if not voiced:
                    continue
                utterance = list(pre_roll)
                speech_frames = 1
                silence_frames = 0
                last_voice_ns = at_ns
                continue
            utterance.append(frame)
            if voiced:
                speech_frames += 1
                silence_frames = 0
                last_voice_ns = at_ns
            else:
                silence_frames += 1
            if len(utterance) < MAX_UTTERANCE_FRAMES and silence_frames < END_SILENCE_FRAMES:
                continue
            pcm = b"".join(utterance)
            eos_ns = last_voice_ns + FRAME_NS
            utterance = []
            pre_roll.clear()
            if speech_frames >= MIN_SPEECH_FRAMES:
                self.transcribe(pcm, eos_ns)
            speech_frames = 0
            silence_frames = 0

    def run(self) -> None:
        threading.Thread(target=self.read_commands, name="voice-stdin", daemon=True).start()
        speaker = self.start_speaker_worker()
        manifest_path = simulated_manifest_path()
        sequence = load_simulated_sequence(manifest_path) if manifest_path else None
        emit(
            "state",
            state="ready",
            stt=STT_REPO,
            stt_revision=STT_REVISION,
            tts=VOICE_NAME,
            tts_en=EN_VOICE_NAME,
            tts_ja=f"Kokoro-82M:{KOKORO_VOICE}",
        )
        if not getattr(self, "listen_enabled", True):
            self.busy = True
            while not self.stop_event.wait(0.03):
                self.handle_commands()
        elif sequence is None:
            with sd.RawInputStream(
                samplerate=SAMPLE_RATE,
                blocksize=FRAME_SAMPLES,
                channels=1,
                dtype="int16",
                callback=self.input_callback,
            ):
                self.resume_listening()
                self.listen_loop()
        else:
            diagnostic(f"using simulated voice input manifest: {manifest_path}")
            self.resume_listening()
            threading.Thread(
                target=self.run_simulated_sequence,
                args=(sequence,),
                name="voice-simulated-input",
                daemon=True,
            ).start()
            self.listen_loop()
        self.shutdown()
        speaker.join(timeout=2.0)
        if speaker.is_alive():
            diagnostic("voice speaker worker did not stop before process exit")
        emit("state", state="stopped")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--setup", action="store_true")
    parser.add_argument("--list-devices", action="store_true")
    parser.add_argument("--no-listen", action="store_true")
    parser.add_argument("--listen-on-command", action="store_true")
    args = parser.parse_args()
    try:
        if args.setup:
            setup_assets()
            return 0
        if args.list_devices:
            print(sd.query_devices())
            return 0
        if args.no_listen and args.listen_on_command:
            raise ValueError("--no-listen and --listen-on-command are mutually exclusive")
        VoiceRuntime(listen_enabled=not args.no_listen, listen_on_command=args.listen_on_command).run()
        return 0
    except Exception as exc:
        diagnostic(f"voice sidecar failed: {exc}")
        emit("state", state="error", detail=str(exc))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
