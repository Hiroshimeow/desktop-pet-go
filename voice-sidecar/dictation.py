"""Local dictation in the existing sidecar: one capture and one serial ASR lease.

Partial hypotheses replace the preview; only an authoritative flushed decode is
final. No clipboard, command, agent, TTS, history or network route lives here.
"""
from __future__ import annotations

import math
import os
import time
import threading
import unicodedata
from typing import Any, Callable

import numpy as np

SAMPLE_RATE = 16_000
FRAME_SAMPLES = 480
PARTIAL_SAMPLES = SAMPLE_RATE * 1200 // 1000
MAX_RECORD_SECONDS = 120
# Keep replay protection for the entire resident lifetime, without an unbounded
# set or evicting IDs that could accidentally reactivate an old session.
MAX_SESSION_IDS = 4096
MUTABLE_TAIL_SECONDS = 0.5
TIMESTAMP_TOLERANCE = 0.35
HOLDBACK_WORDS = 2
MIN_HOLD_SAMPLES = 4000  # 250ms at 16kHz
SILENCE_RMS_FLOOR_DBFS = -50.0
# Ignore sentence punctuation for agreement, never code syntax (+, #, _, etc.)
# or identifier case. The displayed/final transcript itself is not stripped.
PUNCTUATION_SET = ".,!?;:‘’“”…，。、！？；："


class WordAgreement:
    """Two-pass local agreement on whole words at the same audio positions.

    Growing snapshots share the session's audio origin. Never deduplicate by
    suffix strings: identical repeated words at different times are real words.
    A conflicting stable prefix suppresses that preview, not a fabricated merge;
    the authoritative final may still correct any preview.
    """

    def __init__(self) -> None:
        self.previous: list[Any] = []
        self.committed: list[Any] = []
        self.language = ""
        self.horizon = 0.0

    @staticmethod
    def text(words: list[Any]) -> str:
        raw = "".join(word.word for word in words).strip()
        return unicodedata.normalize("NFC", raw)

    @property
    def stable(self) -> str:
        return self.text(self.committed)

    @staticmethod
    def _norm_word(text: str) -> str:
        norm = unicodedata.normalize("NFC", text).strip()
        stripped = norm.strip(PUNCTUATION_SET)
        return stripped if stripped else norm

    @staticmethod
    def matches(a: Any, b: Any) -> bool:
        wa = WordAgreement._norm_word(a.word)
        wb = WordAgreement._norm_word(b.word)
        return (
            wa == wb
            and abs(a.start - b.start) <= TIMESTAMP_TOLERANCE
            and abs(a.end - b.end) <= TIMESTAMP_TOLERANCE
        )

    def update(self, words: list[Any], language: str, horizon: float) -> tuple[str, str] | None:
        if language != self.language:
            self.previous = []
        self.language = language
        if len(words) < len(self.committed) or any(
            not self.matches(a, b) for a, b in zip(self.committed, words)
        ):
            self.previous = []
            self.horizon = horizon
            return None
        count = len(self.committed)
        cutoff = min(self.horizon, horizon) - MUTABLE_TAIL_SECONDS
        # If words length is sufficient, hold back trailing words to avoid prematurely
        # committing words currently being articulated. When sequence is short, cutoff guards.
        max_candidate = max(0, len(words) - HOLDBACK_WORDS) if len(words) >= 4 else len(words)
        while count < min(len(self.previous), max_candidate):
            word = words[count]
            if not self.matches(self.previous[count], word) or word.end > cutoff:
                break
            count += 1
        # Invariant: committed is strictly append-only (cannot shrink)
        if count > len(self.committed):
            self.committed.extend(words[len(self.committed):count])
        # When punctuation differs on the same audio word, retain the existing
        # stable spelling rather than rewriting previously committed text.
        self.previous = words
        self.horizon = horizon
        return self.stable, self.text(words[len(self.committed):])


class DictationStream:
    """Bounded growing-PCM snapshots, newest pending partial, final priority.

    Native faster-whisper generators are fully consumed on one worker. Logical
    cancellation fences publication immediately but retains the lease until the
    native call returns. Forced native deadline/process cleanup belongs to the
    process owner, not a false claim of cooperative native cancellation here.
    """

    def __init__(
        self, model: Any, vad: Any, stop_event: threading.Event, *,
        emit: Callable[..., None], stream_factory: Callable[..., Any],
        max_seconds: float = MAX_RECORD_SECONDS,
        decode_timeout: float = 45.0, cancel_timeout: float = 2.0,
        fatal: Callable[[], None] | None = None,
    ) -> None:
        self.model = model
        self.vad = vad
        self.stop_event = stop_event
        self.emit = emit
        self.stream_factory = stream_factory
        self.max_samples = int(max_seconds * SAMPLE_RATE)
        self.decode_timeout = decode_timeout
        self.cancel_timeout = cancel_timeout
        self.fatal = fatal or (lambda: os._exit(70))
        self.deadline = 0.0
        self.capture_timeout = 2.0
        self.capture_deadline = 0.0
        self.condition = threading.Condition(threading.RLock())
        self.session_id = ""
        self.seen_ids: set[str] = set()
        self.phase = "idle"
        self.pcm = bytearray()
        self.seq = 0
        self.pending_samples = 0
        self.next_partial = PARTIAL_SAMPLES
        self.speech_pending = False
        self.speech_seen = False
        self.failure = ""
        self.stop_reason = ""
        self.capture: Any = None
        self.capture_closing = False
        self.closed = False
        self.worker = threading.Thread(target=self._work, name="dictation-asr", daemon=True)
        self.worker.start()
        self.watchdog = threading.Thread(target=self._watch_deadline, name="dictation-deadline", daemon=True)
        self.watchdog.start()

    def _watch_deadline(self) -> None:
        expired = False
        with self.condition:
            while not self.closed:
                deadlines = [d for d in (self.deadline, self.capture_deadline) if d]
                if not deadlines:
                    self.condition.wait()
                    continue
                remaining = min(deadlines) - time.monotonic()
                if remaining > 0:
                    self.condition.wait(remaining)
                    continue
                # Native CTranslate2 cannot be safely interrupted/reused. Fence
                # publication and retire this process, not start a competing ASR.
                self._emit("error", code="native_timeout", reason="Audio operation timed out; no text inserted")
                self._emit("done", outcome="error")
                expired = True
                self.closed = True
                self.stop_event.set()
                self.condition.notify_all()
                break
        if expired:
            self.fatal()

    @property
    def active(self) -> bool:
        with self.condition:
            return self.phase != "idle"

    def _emit(self, kind: str, **payload: Any) -> None:
        # Called under the session lock: sequence and terminal transitions cannot
        # race cancel/restart. stdout backpressure is a process-owner concern.
        if not self.closed and not self.stop_event.is_set():
            self.seq += 1
            self.emit(kind, protocol_version=1, session_id=self.session_id, seq=self.seq, **payload)

    def start(self, session_id: str) -> bool:
        with self.condition:
            if not session_id or len(session_id) > 128 or session_id in self.seen_ids or self.active or self.closed or self.stop_event.is_set():
                return False
            self.session_id = session_id
            if len(self.seen_ids) >= MAX_SESSION_IDS:
                self.seq = 0
                self._emit("error", code="session_limit", reason="Local voice session limit reached; restart local voice. No text inserted")
                self._emit("done", outcome="error")
                return True  # Accepted envelope, terminal failure; no capture.
            self.seen_ids.add(session_id)
            self.phase = "recording"
            self.pcm = bytearray()
            self.seq = 0
            self.pending_samples = 0
            self.next_partial = PARTIAL_SAMPLES
            self.speech_pending = self.speech_seen = False
            self.failure = self.stop_reason = ""
            self.agreement = WordAgreement()
            try:
                self.capture = self.stream_factory(
                    samplerate=SAMPLE_RATE, blocksize=FRAME_SAMPLES, channels=1,
                    dtype="int16", callback=self.input_callback,
                )
                self._emit("record_started")
                self.capture.start()
            except Exception:
                self.failure = "capture_failed"
                self.phase = "error"
            self.condition.notify_all()
            return True

    def input_callback(self, indata: Any, frames: int, _time_info: Any, status: Any) -> None:
        with self.condition:
            if self.phase not in ("recording", "stopping") or self.closed or self.stop_event.is_set():
                return
            frame = bytes(indata)
            if status or frames != FRAME_SAMPLES or len(frame) != FRAME_SAMPLES * 2:
                self.failure = "audio_discontinuity"
                self.phase = "error"
            else:
                # Keep ALL owned PCM, including leading context and stop tail.
                # No busy gate, VAD crop, queue-drop or string-based seam splice.
                remaining = self.max_samples * 2 - len(self.pcm)
                if len(frame) > remaining:
                    # A draining capture must never exceed the same RAM bound.
                    # Fail closed rather than silently truncating stop-tail speech.
                    self.failure = "audio_limit"
                    self.phase = "error"
                    self.pending_samples = 0
                    self.condition.notify_all()
                    return
                self.pcm.extend(frame)
                samples_array = np.frombuffer(frame, dtype=np.int16)
                audible = int(np.max(np.abs(samples_array.astype(np.int32)))) >= 64
                if not self.speech_seen and audible and self.vad.is_speech(frame, SAMPLE_RATE):
                    self.speech_seen = self.speech_pending = True
                samples = len(self.pcm) // 2
                if samples >= self.max_samples and self.phase == "recording":
                    self.phase = "limit"
                    self.stop_reason = "duration_limit"
                elif self.phase == "recording" and self.speech_seen and samples >= self.next_partial:
                    self.pending_samples = samples
                    self.next_partial = samples + PARTIAL_SAMPLES
            self.condition.notify_all()

    def _close_capture(self) -> None:
        with self.condition:
            self.condition.wait_for(lambda: not self.capture_closing)
            capture, self.capture = self.capture, None
            self.capture_closing = capture is not None
            if capture is not None:
                self.capture_deadline = time.monotonic() + self.capture_timeout
                self.condition.notify_all()
        if capture is not None:
            try:
                try:
                    capture.stop()
                finally:
                    capture.close()
            finally:
                with self.condition:
                    self.capture_closing = False
                    self.capture_deadline = 0.0
                    self.condition.notify_all()

    def stop(self, session_id: str) -> bool:
        with self.condition:
            if session_id != self.session_id or self.phase == "idle":
                return False
            if self.phase != "recording":
                return True
            self.phase = "stopping"
            self.pending_samples = 0
        # PortAudio stop waits for the last callback; accept its PCM before the
        # final snapshot. Never hold the lock while joining capture callbacks.
        try:
            self._close_capture()
        except Exception:
            with self.condition:
                self.failure = "capture_failed"
                self.phase = "error"
        with self.condition:
            if self.phase == "stopping":
                total_samples = len(self.pcm) // 2
                if not self.speech_seen and total_samples < MIN_HOLD_SAMPLES:
                    # Tap was too short (< 250ms) with no detected speech: cancel with reason "too_short"
                    self.phase = "cancelled"
                    self.stop_reason = "too_short"
                else:
                    self.phase = "finalizing"
                    self.stop_reason = "stop"
            self.pending_samples = 0
            self.condition.notify_all()
        return True

    def cancel(self, session_id: str) -> bool:
        with self.condition:
            if session_id != self.session_id or self.phase == "idle":
                return False
            self.phase = "cancelled"
            if self.deadline:
                self.deadline = min(self.deadline, time.monotonic() + self.cancel_timeout)
            self.pending_samples = 0
            self.condition.notify_all()
        self._close_capture()
        return True

    def shutdown(self) -> None:
        with self.condition:
            self.closed = True
            self.phase = "cancelled" if self.active else "idle"
            self.condition.notify_all()
        self._close_capture()

    def join(self, timeout: float) -> bool:
        started = time.monotonic()
        self.worker.join(timeout)
        self.watchdog.join(max(0.0, timeout - (time.monotonic() - started)))
        return not self.worker.is_alive() and not self.watchdog.is_alive()

    def _decode(self, pcm: bytes) -> tuple[str, str, list[Any]]:
        if not pcm:
            return "", "", []
        raw_samples = np.frombuffer(pcm, dtype=np.int16)
        if raw_samples.size == 0:
            return "", "", []
        # Audio RMS floor check: if RMS < -50 dBFS, treat as silence, return empty
        # to avoid Faster-Whisper hallucinations on near-zero acoustic energy.
        float_samples = raw_samples.astype(np.float32) / 32768.0
        rms = float(np.sqrt(np.mean(float_samples ** 2)))
        rms_dbfs = 20.0 * math.log10(max(rms, 1e-9))
        if rms_dbfs < SILENCE_RMS_FLOOR_DBFS:
            return "", "", []

        segments, info = self.model.transcribe(
            float_samples, beam_size=1, best_of=1, temperature=0,
            condition_on_previous_text=False, vad_filter=False, word_timestamps=True,
        )
        segments = list(segments)  # The actual native decode is lazy.
        text = " ".join(segment.text.strip() for segment in segments).strip()
        timed = [word for segment in segments for word in (segment.words or [])]
        # Bad/missing timing can still produce a provisional preview, never
        # stable agreement. Never normalize accents, punctuation or identifiers.
        if any(
            not math.isfinite(word.start) or not math.isfinite(word.end)
            or word.start < 0 or word.end < word.start
            or (index > 0 and word.start < timed[index - 1].start)
            for index, word in enumerate(timed)
        ):
            timed = []
        return text, info.language, timed

    def _finish(self, outcome: str) -> None:
        self._emit("done", outcome=outcome)
        self.phase = "idle"
        self.pending_samples = 0
        self.pcm.clear()
        self.condition.notify_all()

    def _work(self) -> None:
        while True:
            with self.condition:
                self.condition.wait_for(lambda: self.closed or self.speech_pending or self.pending_samples or self.phase in ("finalizing", "limit", "cancelled", "error"))
                if self.closed or self.stop_event.is_set():
                    return
                if self.speech_pending:
                    self.speech_pending = False
                    self._emit("speech_start")
                phase = self.phase
                if phase in ("cancelled", "error", "limit"):
                    # Close outside lock; callback may be waiting on this lock.
                    pass
                elif phase == "finalizing":
                    pcm = bytes(self.pcm) if self.speech_seen else b""
                    self.pending_samples = 0
                    self._emit("record_stopped", reason=self.stop_reason)
                elif self.pending_samples:
                    pcm = bytes(self.pcm[:self.pending_samples * 2])
                    self.pending_samples = 0
                else:
                    continue
            if phase in ("cancelled", "error", "limit"):
                try:
                    self._close_capture()
                except Exception:
                    with self.condition:
                        self.failure = "capture_failed"
                        self.phase = "error"
                with self.condition:
                    if self.phase == "limit":
                        self.phase = "finalizing"
                    elif self.phase == "cancelled":
                        if getattr(self, "stop_reason", "") == "too_short":
                            self._emit("cancelled", reason="too_short")
                        else:
                            self._emit("cancelled")
                        self._finish("cancelled")
                    elif self.phase == "error":
                        self._emit("error", code=self.failure, reason="Recording or decoding failed; no text inserted")
                        self._finish("error")
                continue
            with self.condition:
                if self.closed or self.phase != phase:
                    continue
                self.deadline = time.monotonic() + self.decode_timeout
                self.condition.notify_all()
            try:
                text, language, timed = self._decode(pcm) if pcm else ("", "", [])
            except Exception:
                with self.condition:
                    # A stop request must not turn a real native failure into
                    # silent retry/success. Cancel/shutdown still take precedence.
                    if not self.closed and self.phase in ("recording", "stopping", "finalizing", "limit"):
                        self.phase = "error"
                        self.failure = "decode_failed"
                continue
            finally:
                with self.condition:
                    self.deadline = 0.0
                    self.condition.notify_all()
            with self.condition:
                if self.closed or self.stop_event.is_set() or self.phase != phase:
                    continue
                if phase == "finalizing":
                    self._emit("final", text=text, language=language)
                    self._finish("final")
                else:
                    preview = self.agreement.update(timed, language, len(pcm) / 2 / SAMPLE_RATE)
                    if preview is not None:
                        stable, unstable = preview
                        if not timed:
                            unstable = text
                        self._emit("partial", stable=stable, unstable=unstable, language=language, audio_end_ms=len(pcm) * 1000 // (2 * SAMPLE_RATE))
