"""Deterministic product tests; fake only the native ASR/microphone boundary."""
import subprocess
import sys
import threading
import types
import unittest
from unittest.mock import patch

import numpy as np

try:
    import dictation
except ModuleNotFoundError:
    dictation = None


FRAME = np.full(480, 512, dtype=np.int16).tobytes()


def words(*items):
    return [types.SimpleNamespace(word=text, start=start, end=end) for text, start, end in items]


def result(text, timed_words=None, language="vi"):
    return [types.SimpleNamespace(text=text, words=timed_words)], types.SimpleNamespace(language=language)


class StreamingTest(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(dictation, "session-fenced dictation implementation is missing")
        self.events = []
        self.changed = threading.Condition()
        self.streams = []
        self.stop = threading.Event()
        self.engines = []

    def emit(self, kind, **payload):
        with self.changed:
            self.events.append({"type": kind, **payload})
            self.changed.notify_all()

    def wait_type(self, kind):
        with self.changed:
            self.assertTrue(self.changed.wait_for(lambda: any(e["type"] == kind for e in self.events), 3), self.events)
        return next(e for e in self.events if e["type"] == kind)

    def make_engine(self, model, *, max_seconds=120):
        owner = self

        class Capture:
            def __init__(self, **kwargs):
                self.callback = kwargs["callback"]
                self.closed = False
                owner.streams.append(self)

            def start(self):
                return self

            def stop(self):
                if getattr(self, "tail", None) is not None:
                    self.callback(self.tail, 480, None, None)
                    self.tail = None

            def close(self):
                self.closed = True

        vad = types.SimpleNamespace(is_speech=lambda frame, rate: bool(np.frombuffer(frame, np.int16).max()))
        engine = dictation.DictationStream(model, vad, self.stop, emit=self.emit, stream_factory=Capture, max_seconds=max_seconds)
        self.engines.append(engine)
        return engine

    def tearDown(self):
        for engine in getattr(self, "engines", []):
            engine.shutdown()
            self.assertTrue(engine.join(3), "test worker leaked")

    def feed(self, engine, count, frame=FRAME):
        for _ in range(count):
            engine.input_callback(frame, 480, None, None)

    def test_first_word_and_overlapping_snapshots_are_replacements_not_concatenation(self):
        calls = []

        class Model:
            def transcribe(_, audio, **kwargs):
                calls.append((audio.copy(), kwargs))
                return result(" Xin chào API_name", words((" Xin", .1, .25), (" chào", .3, .45), (" API_name", .5, .7)))

        engine = self.make_engine(Model())
        self.assertTrue(engine.start("first"))
        # First audible sample must not be gated away or appended twice.
        first = np.full(480, 2048, dtype=np.int16).tobytes()
        self.feed(engine, 1, first)
        self.feed(engine, 39)
        partial = self.wait_type("partial")
        self.assertEqual(partial["unstable"], "Xin chào API_name")
        self.assertEqual(partial["stable"], "")
        self.assertTrue(np.all(calls[0][0][:480] == 2048 / 32768))
        self.feed(engine, 40)
        with self.changed:
            self.assertTrue(self.changed.wait_for(lambda: len([e for e in self.events if e["type"] == "partial"]) >= 2, 3))
        self.assertTrue(engine.stop("first"))
        final = self.wait_type("final")
        self.wait_type("done")
        self.assertEqual(final["text"], "Xin chào API_name")
        self.assertEqual(len(calls[-1][0]), 80 * 480)
        self.assertTrue(all(c[1]["word_timestamps"] for c in calls))
        self.assertTrue(all("language" not in c[1] for c in calls))
        self.assertFalse(any(e["type"] in ("utterance", "speak", "paste") for e in self.events))
        self.assertTrue(all(e["protocol_version"] == 1 and e["session_id"] == "first" for e in self.events))
        self.assertEqual([e["seq"] for e in self.events], list(range(1, len(self.events) + 1)))

    def test_slow_decode_keeps_capture_and_only_newest_pending_partial(self):
        entered = threading.Event()
        release = threading.Event()
        sizes = []

        class Model:
            def transcribe(_, audio, **kwargs):
                sizes.append(len(audio))
                if len(sizes) == 1:
                    entered.set()
                    self.assertTrue(release.wait(3))
                return result("Hello")

        engine = self.make_engine(Model())
        engine.start("latest")
        self.feed(engine, 40)
        self.assertTrue(entered.wait(3))
        self.feed(engine, 120)
        release.set()
        with self.changed:
            self.assertTrue(self.changed.wait_for(lambda: len([e for e in self.events if e["type"] == "partial"]) >= 2, 3))
        self.assertEqual(sizes, [40 * 480, 160 * 480])
        engine.stop("latest")
        self.wait_type("done")

    def test_stop_flushes_tail_prioritizes_final_and_rejects_stale_partial(self):
        entered = threading.Event()
        release = threading.Event()
        sizes = []

        class Model:
            def transcribe(_, audio, **kwargs):
                sizes.append(len(audio))
                if len(sizes) == 1:
                    entered.set()
                    self.assertTrue(release.wait(3))
                    return result("stale preview")
                return result("Tiếng Việt and UTF_8 ✓")

        engine = self.make_engine(Model())
        engine.start("flush")
        self.feed(engine, 40)
        self.assertTrue(entered.wait(3))
        self.feed(engine, 45)
        engine.stop("old-session")
        self.assertFalse(self.streams[0].closed)
        engine.stop("flush")
        engine.stop("flush")
        self.assertTrue(self.streams[0].closed)
        self.feed(engine, 10)
        release.set()
        final = self.wait_type("final")
        self.wait_type("done")
        self.assertEqual(final["text"], "Tiếng Việt and UTF_8 ✓")
        self.assertEqual(sizes, [40 * 480, 85 * 480])
        self.assertFalse(any(e["type"] == "partial" for e in self.events))
        self.assertEqual(len([e for e in self.events if e["type"] == "final"]), 1)

    def test_cancel_fences_lazy_result_and_retains_model_lease_until_return(self):
        entered = threading.Event()
        release = threading.Event()

        class Model:
            def transcribe(_, audio, **kwargs):
                def segments():
                    entered.set()
                    self.assertTrue(release.wait(3))
                    yield types.SimpleNamespace(text="must not publish", words=None)
                return segments(), types.SimpleNamespace(language="en")

        engine = self.make_engine(Model())
        engine.start("cancel")
        self.feed(engine, 40)
        self.assertTrue(entered.wait(3))
        self.assertFalse(engine.cancel("old"))
        self.assertTrue(engine.cancel("cancel"))
        self.assertTrue(engine.cancel("cancel"))
        self.assertFalse(engine.start("next"))
        self.assertTrue(self.streams[0].closed)
        release.set()
        done = self.wait_type("done")
        self.assertEqual(done["outcome"], "cancelled")
        self.assertFalse(any(e["type"] in ("partial", "final") for e in self.events))
        self.assertEqual(len([e for e in self.events if e["type"] == "cancelled"]), 1)
        self.assertFalse(engine.start("cancel"), "session IDs must not be reused")
        self.assertTrue(engine.start("next"))
        self.assertFalse(engine.stop("cancel"))
        engine.cancel("next")

    def test_capture_stop_drains_last_callback_and_cannot_publish_preview_after_stop(self):
        sizes = []

        def transcribe(audio, **kwargs):
            sizes.append(len(audio))
            return result("tail")

        engine = self.make_engine(types.SimpleNamespace(transcribe=transcribe))
        engine.start("tail")
        self.feed(engine, 1)
        self.streams[0].tail = FRAME
        engine.stop("tail")
        self.wait_type("done")
        self.assertEqual(sizes, [960])

    def test_stop_request_fences_preview_while_capture_drains(self):
        entered = threading.Event()
        release = threading.Event()
        draining = threading.Event()
        drain_release = threading.Event()

        def transcribe(audio, **kwargs):
            if not entered.is_set():
                entered.set()
                self.assertTrue(release.wait(3))
            return result("preview")

        engine = self.make_engine(types.SimpleNamespace(transcribe=transcribe))
        engine.start("draining")
        self.feed(engine, 40)
        self.assertTrue(entered.wait(3))

        def stop_capture():
            draining.set()
            self.assertTrue(drain_release.wait(3))
            engine.input_callback(FRAME, 480, None, None)

        self.streams[0].stop = stop_capture
        stopper = threading.Thread(target=lambda: engine.stop("draining"))
        stopper.start()
        self.assertTrue(draining.wait(3))
        release.set()
        # The worker must not publish the finished recording-phase hypothesis
        # once a stop request has fenced it, even while the device drains.
        with engine.condition:
            self.assertEqual(engine.phase, "stopping")
        drain_release.set()
        stopper.join(3)
        self.wait_type("done")
        self.assertFalse(any(e["type"] == "partial" for e in self.events))

    def test_decode_and_capture_failures_publish_one_error_done_without_final(self):
        def transcribe(audio, **kwargs):
            raise RuntimeError("private backend details must not leak")

        engine = self.make_engine(types.SimpleNamespace(transcribe=transcribe))
        engine.start("decode-error")
        self.feed(engine, 1)
        engine.stop("decode-error")
        self.assertEqual(self.wait_type("error")["code"], "decode_failed")
        self.wait_type("done")
        self.assertFalse(any(e["type"] == "final" for e in self.events))
        self.assertNotIn("private backend", str(self.events))
        self.events.clear()
        engine.stream_factory = lambda **kw: (_ for _ in ()).throw(OSError("private device name"))
        engine.start("capture-error")
        self.assertEqual(self.wait_type("error")["code"], "capture_failed")
        self.wait_type("done")

    def test_cancel_completion_waits_for_capture_close_before_next_activation(self):
        closing = threading.Event()
        release = threading.Event()
        engine = self.make_engine(types.SimpleNamespace(transcribe=lambda *a, **kw: result("unused")))
        engine.start("close-lease")

        def close():
            closing.set()
            self.assertTrue(release.wait(3))

        self.streams[0].close = close
        canceller = threading.Thread(target=lambda: engine.cancel("close-lease"))
        canceller.start()
        self.assertTrue(closing.wait(3))
        try:
            self.assertFalse(engine.start("new-owner"))
            self.assertFalse(any(e["type"] == "done" for e in self.events))
        finally:
            release.set()
            canceller.join(3)
        self.wait_type("done")

    def test_silent_recording_never_decodes_hallucinated_text(self):
        def transcribe(*args, **kwargs):
            self.fail("silence must not be sent to the recognizer")

        engine = self.make_engine(types.SimpleNamespace(transcribe=transcribe))
        engine.start("silence")
        self.feed(engine, 40, bytes(960))
        engine.stop("silence")
        self.assertEqual(self.wait_type("final")["text"], "")
        self.wait_type("done")

    def test_device_discontinuity_fails_closed_without_final(self):
        engine = self.make_engine(types.SimpleNamespace(transcribe=lambda *a, **kw: result("unsafe")))
        engine.start("overflow")
        engine.input_callback(FRAME, 480, None, "input overflow")
        self.assertEqual(self.wait_type("error")["code"], "audio_discontinuity")
        self.assertEqual(self.wait_type("done")["outcome"], "error")
        self.assertFalse(any(e["type"] == "final" for e in self.events))

    def test_short_tap_without_speech_cancels_as_too_short(self):
        engine = self.make_engine(types.SimpleNamespace(transcribe=lambda *a, **kw: result("unused")))
        engine.start("tap")
        # 1 frame = 30ms = 480 samples < MIN_HOLD_SAMPLES (4000), silent frame
        engine.input_callback(bytes(960), 480, None, None)
        engine.stop("tap")
        cancelled = self.wait_type("cancelled")
        self.assertEqual(cancelled.get("reason"), "too_short")
        done = self.wait_type("done")
        self.assertEqual(done["outcome"], "cancelled")
        self.assertFalse(any(e["type"] == "final" for e in self.events))

    def test_limit_is_visible_and_finalizes_owned_audio_without_silent_truncation(self):
        sizes = []

        def transcribe(audio, **kwargs):
            sizes.append(len(audio))
            return result("bounded")

        engine = self.make_engine(types.SimpleNamespace(transcribe=transcribe), max_seconds=.06)
        engine.start("limit")
        self.feed(engine, 3)
        self.wait_type("done")
        self.assertEqual(sizes, [960])
        self.assertTrue(any(e["type"] == "record_stopped" and e["reason"] == "duration_limit" for e in self.events))

    def test_partial_decode_failure_raced_with_stop_fails_closed(self):
        entered = threading.Event()
        release = threading.Event()
        calls = []

        def transcribe(audio, **kwargs):
            calls.append(len(audio))
            if len(calls) == 1:
                entered.set()
                self.assertTrue(release.wait(3))
                raise RuntimeError("native partial failed")
            return result("must not silently retry and paste")

        engine = self.make_engine(types.SimpleNamespace(transcribe=transcribe))
        engine.start("failed-stop")
        self.feed(engine, 40)
        self.assertTrue(entered.wait(3))
        engine.stop("failed-stop")
        release.set()
        self.wait_type("done")
        self.assertEqual([e["type"] for e in self.events][-2:], ["error", "done"])
        self.assertEqual(calls, [19200])
        self.assertFalse(any(e["type"] == "final" for e in self.events))

    def test_cancel_during_capture_stop_is_not_overwritten_by_short_tap(self):
        draining = threading.Event()
        release = threading.Event()
        engine = self.make_engine(types.SimpleNamespace(transcribe=lambda *a, **kw: result("unsafe")))
        engine.start("cancel-stop")

        def stop_capture():
            draining.set()
            self.assertTrue(release.wait(3))

        self.streams[0].stop = stop_capture
        stopper = threading.Thread(target=lambda: engine.stop("cancel-stop"))
        stopper.start()
        self.assertTrue(draining.wait(3))
        canceller = threading.Thread(target=lambda: engine.cancel("cancel-stop"))
        canceller.start()
        release.set()
        stopper.join(3)
        canceller.join(3)
        self.wait_type("done")
        self.assertEqual(len([e for e in self.events if e["type"] == "cancelled"]), 1)
        self.assertFalse(any(e["type"] == "final" for e in self.events))

    def test_audio_bound_includes_stop_tail(self):
        engine = self.make_engine(types.SimpleNamespace(transcribe=lambda *a, **kw: result("bounded")), max_seconds=.06)
        engine.start("bounded-tail")
        self.feed(engine, 1)
        # Device may deliver more than one callback while its stop drains.
        def stop_capture():
            self.feed(engine, 20)
            with engine.condition:
                self.assertLessEqual(len(engine.pcm), engine.max_samples * 2)
        self.streams[0].stop = stop_capture
        engine.stop("bounded-tail")
        self.wait_type("done")
        self.assertTrue(any(e["type"] == "error" and e["code"] == "audio_limit" for e in self.events))
        self.assertFalse(any(e["type"] == "final" for e in self.events))

    def test_hung_capture_close_has_bounded_owner_retirement(self):
        entered = threading.Event()
        release = threading.Event()
        retired = threading.Event()
        engine = self.make_engine(types.SimpleNamespace(transcribe=lambda *a, **kw: result("unused")))
        engine.capture_timeout = .05
        engine.fatal = retired.set
        engine.start("capture-hang")

        def close():
            entered.set()
            release.wait(3)

        self.streams[0].close = close
        canceller = threading.Thread(target=lambda: engine.cancel("capture-hang"))
        canceller.start()
        self.assertTrue(entered.wait(1))
        try:
            self.assertTrue(retired.wait(1), "capture close had no retirement deadline")
            self.assertFalse(engine.start("unsafe-new-owner"))
        finally:
            release.set()
            canceller.join(3)

    def test_native_deadline_fences_final_and_retires_owner(self):
        entered = threading.Event()
        release = threading.Event()
        expired = threading.Event()

        def transcribe(*args, **kwargs):
            entered.set()
            self.assertTrue(release.wait(3))
            return result("late native result")

        engine = self.make_engine(types.SimpleNamespace(transcribe=transcribe))
        engine.decode_timeout = .05
        engine.cancel_timeout = .05
        engine.fatal = lambda: expired.set()
        engine.start("timeout")
        self.feed(engine, 40)
        self.assertTrue(entered.wait(3))
        try:
            engine.cancel("timeout")
            self.assertTrue(expired.wait(1), "native lease had no bounded retirement deadline")
            self.assertFalse(engine.start("competing-owner"))
        finally:
            release.set()
        self.assertTrue(engine.join(3))
        self.assertFalse(any(e["type"] == "final" for e in self.events))

    def test_rapid_toggle_reuses_one_model_without_duplicate_final(self):
        calls = []
        model = types.SimpleNamespace(transcribe=lambda audio, **kw: calls.append(len(audio)) or result("Hello"))
        engine = self.make_engine(model)
        for index in range(40):
            sid = f"rapid-{index}"
            before = len(self.events)
            self.assertTrue(engine.start(sid))
            self.feed(engine, 10)
            self.assertTrue(engine.stop(sid))
            engine.stop(sid)
            with self.changed:
                self.assertTrue(self.changed.wait_for(lambda: any(e["type"] == "done" and e["session_id"] == sid for e in self.events[before:]), 3))
            owned = [e for e in self.events[before:] if e["session_id"] == sid]
            self.assertEqual(sum(e["type"] == "final" for e in owned), 1)
            self.assertEqual(sum(e["type"] == "done" for e in owned), 1)
            self.assertIs(engine.model, model)
        self.assertEqual(len(calls), 40)
        self.assertTrue(all(s.closed for s in self.streams))

    def test_shutdown_fences_inflight_final(self):
        entered = threading.Event()
        release = threading.Event()

        def transcribe(audio, **kwargs):
            entered.set()
            self.assertTrue(release.wait(3))
            return result("late")

        engine = self.make_engine(types.SimpleNamespace(transcribe=transcribe))
        engine.start("shutdown")
        self.feed(engine, 1)
        engine.stop("shutdown")
        self.assertTrue(entered.wait(3))
        self.stop.set()
        engine.shutdown()
        release.set()
        self.assertTrue(engine.join(3))
        self.assertFalse(any(e["type"] == "final" for e in self.events))


class ProcessDeadlineTest(unittest.TestCase):
    def test_stuck_native_decoder_exits_real_sidecar_process(self):
        # Real interpreter retirement through os._exit; native decode alone is
        # substituted with an endless wait, without touching microphone/audio.
        code = """
import threading, types, numpy as np
from dictation import DictationStream
class Capture:
    def __init__(self, **kw): pass
    def start(self): pass
    def stop(self): pass
    def close(self): pass
def stuck(*args, **kwargs):
    threading.Event().wait()
engine = DictationStream(types.SimpleNamespace(transcribe=stuck),
    types.SimpleNamespace(is_speech=lambda *a: True), threading.Event(),
    emit=lambda *a, **kw: None, stream_factory=Capture, decode_timeout=.1)
engine.start('deadline')
frame = np.full(480, 512, dtype=np.int16).tobytes()
for _ in range(40): engine.input_callback(frame, 480, None, None)
threading.Event().wait()
"""
        process = subprocess.Popen([sys.executable, "-c", code])
        try:
            self.assertEqual(process.wait(timeout=8), 70)
        finally:
            if process.poll() is None:
                process.kill()
                process.wait(timeout=3)


class AgreementTest(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(dictation, "timestamp-local agreement implementation is missing")
        self.agreement = dictation.WordAgreement()

    def test_two_timestamp_aligned_hypotheses_stabilize_whole_unicode_words(self):
        hypothesis = words((" Xin", .1, .3), (" chào", .35, .6), (" API_name", .7, 1.2))
        self.assertEqual(self.agreement.update(hypothesis, "vi", 1.3), ("", "Xin chào API_name"))
        self.assertEqual(self.agreement.update(hypothesis, "vi", 2), ("Xin chào", "API_name"))
        self.assertEqual(self.agreement.update(hypothesis, "vi", 3), ("Xin chào API_name", ""))

    def test_repeated_text_at_different_audio_positions_does_not_stabilize(self):
        self.agreement.update(words((" go", .1, .3)), "en", 2)
        self.assertEqual(self.agreement.update(words((" go", 1.1, 1.3)), "en", 3), ("", "go"))

    def test_language_flip_blank_and_stable_conflict_do_not_reuse_stale_agreement(self):
        original = words((" Xin", .1, .3), (" chào", .4, .6))
        self.agreement.update(original, "en", 2)
        self.assertEqual(self.agreement.update(original, "vi", 3), ("", "Xin chào"))
        self.agreement.update([], "vi", 4)
        self.assertEqual(self.agreement.update(original, "vi", 5), ("", "Xin chào"))
        self.assertEqual(self.agreement.update(original, "vi", 6), ("Xin chào", ""))
        self.assertIsNone(self.agreement.update(words((" wrong", .1, .3)), "vi", 7))
        self.assertEqual(self.agreement.stable, "Xin chào")

    def test_growing_overlap_preserves_real_repetition_and_does_not_commit_subwords(self):
        first = words((" go", .1, .3), (" go", .4, .6), (" API", .7, .9))
        self.agreement.update(first, "en", 2)
        second = words((" go", .1, .3), (" go", .4, .6), (" API_name", .7, .9))
        self.assertEqual(self.agreement.update(second, "en", 3), ("go go", "API_name"))

    def test_committed_unicode_prefix_keeps_its_spelling_when_agreement_grows(self):
        initial = words((" Xin,", .1, .3), (" chào!", .4, .6))
        self.agreement.update(initial, "vi", 2)
        self.assertEqual(self.agreement.update(initial, "vi", 3), ("Xin, chào!", ""))
        revised = words((" Xin", .1, .3), (" chào", .4, .6), (" API_name", .7, .9))
        self.agreement.update(revised, "vi", 4)
        stable, unstable = self.agreement.update(revised, "vi", 5)
        self.assertEqual(stable, "Xin, chào! API_name")
        self.assertEqual(unstable, "")

    def test_identifier_suffixes_and_case_changes_are_not_word_agreement(self):
        for old, new in ((" C++", " C#"), (" __name__", " name"), (" API_name", " api_name")):
            with self.subTest(old=old, new=new):
                agreement = dictation.WordAgreement()
                agreement.update(words((old, .1, .3)), "en", 2)
                self.assertEqual(agreement.update(words((new, .1, .3)), "en", 3), ("", new.strip()))

    def test_unicode_nfc_and_punctuation_and_timestamp_tolerance_matches(self):
        import unicodedata
        # NFD Vietnamese text
        nfd_word = unicodedata.normalize("NFD", "chào")
        h1 = words((" Xin,", .10, .30), (" " + nfd_word + "!", .35, .60))
        self.agreement.update(h1, "vi", 1.5)
        # NFC Vietnamese text with 0.25s timestamp jitter (within 0.35s tolerance)
        nfc_word = unicodedata.normalize("NFC", "chào")
        h2 = words((" Xin", .20, .40), (" " + nfc_word, .50, .75))
        stable, unstable = self.agreement.update(h2, "vi", 2.0)
        self.assertEqual(self.agreement.stable, "Xin chào")



if __name__ == "__main__":
    unittest.main()
