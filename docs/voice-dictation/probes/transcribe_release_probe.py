"""G04 native reuse probe, NOT product binding/accuracy acceptance.

Run in a bounded child process (120s outer timeout); no microphone, network,
clipboard, user settings, or native app launch. Requires existing numpy.
Exact public transcribe.cpp 0.2.4 ABI; do not load mismatched providers.

Struct declarations derived from transcribe.cpp include/transcribe.h:
MIT License
Copyright (c) 2026 The transcribe.cpp authors
Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:
The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.
THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
"""
from pathlib import Path
import argparse
import ctypes as c
import hashlib
import json
import os
import sys
import time
import wave

import numpy as np


class Load(c.Structure):
    _fields_ = [("struct_size", c.c_uint64), ("backend", c.c_int), ("device", c.c_void_p)]


class Update(c.Structure):
    _fields_ = [
        ("struct_size", c.c_uint64), ("result_changed", c.c_bool),
        ("is_final", c.c_bool), ("revision", c.c_int32),
        ("input_received_ms", c.c_int64), ("audio_committed_ms", c.c_int64),
        ("buffered_ms", c.c_int64), ("committed_changed", c.c_bool),
        ("tentative_changed", c.c_bool),
    ]


class Text(c.Structure):
    _fields_ = [
        ("struct_size", c.c_uint64), ("full_text", c.c_char_p),
        ("full_text_bytes", c.c_uint64), ("committed_text", c.c_char_p),
        ("committed_text_bytes", c.c_uint64), ("tentative_text", c.c_char_p),
        ("tentative_text_bytes", c.c_uint64), ("raw_tentative_start_bytes", c.c_uint64),
    ]


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--dll-dir", type=Path, required=True)
    parser.add_argument("--model", type=Path, required=True)
    parser.add_argument("--audio", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True, help="Private JSON, includes transcript")
    args = parser.parse_args()
    sys.stdout.reconfigure(encoding="utf-8")
    with wave.open(str(args.audio), "rb") as wav:
        assert (wav.getnchannels(), wav.getsampwidth(), wav.getframerate()) == (1, 2, 16000)
        assert 0 < wav.getnframes() <= 16000 * 5, "Use short synthetic fixture, not private mic audio"
        audio = np.frombuffer(wav.readframes(wav.getnframes()), dtype="<i2").astype(np.float32) / 32768
    audio = np.concatenate([audio, np.zeros(6400, np.float32)] * 4)
    dll_dir = args.dll_dir.resolve()
    # Keep directory handle alive throughout all native calls/teardown.
    with os.add_dll_directory(str(dll_dir)):
        lib = c.CDLL(str(dll_dir / "transcribe.dll"))

        def bind(name, arguments, result=c.c_int):
            function = getattr(lib, name)
            function.argtypes, function.restype = arguments, result
            return function

        ptr = c.c_void_p
        version = bind("transcribe_version", [], c.c_char_p)().decode()
        assert version == "0.2.4", version
        abi = bind("transcribe_abi_struct_size", [c.c_int], c.c_size_t)
        align = bind("transcribe_abi_struct_align", [c.c_int], c.c_size_t)
        for tag, cls in [(0, Load), (9, Update), (10, Text)]:
            assert abi(tag) == c.sizeof(cls) and align(tag) == c.alignment(cls), cls.__name__
        init_backends = bind("transcribe_init_backends", [c.c_char_p])
        load_init = bind("transcribe_model_load_params_init", [c.POINTER(Load)], None)
        open_session = bind("transcribe_open", [c.c_char_p, c.POINTER(Load), ptr, c.POINTER(ptr)])
        close = bind("transcribe_session_free", [ptr], None)
        get_model = bind("transcribe_get_model", [ptr], ptr)
        backend = bind("transcribe_model_backend", [ptr], c.c_char_p)
        begin = bind("transcribe_stream_begin", [ptr, ptr, ptr])
        feed = bind("transcribe_stream_feed", [ptr, c.POINTER(c.c_float), c.c_int, c.POINTER(Update)])
        finalize = bind("transcribe_stream_finalize", [ptr, c.POINTER(Update)])
        reset = bind("transcribe_stream_reset", [ptr], None)
        state = bind("transcribe_stream_get_state", [ptr])
        update_init = bind("transcribe_stream_update_init", [c.POINTER(Update)], None)
        text_init = bind("transcribe_stream_text_init", [c.POINTER(Text)], None)
        get_text = bind("transcribe_stream_get_text", [ptr, c.POINTER(Text)])
        abort_type = c.CFUNCTYPE(c.c_bool, ptr)
        set_abort = bind("transcribe_set_abort_callback", [ptr, abort_type, ptr], None)
        was_aborted = bind("transcribe_was_aborted", [ptr], c.c_bool)
        abort_flag = False

        @abort_type
        def abort(_):
            return abort_flag

        def snapshot(session):
            text = Text()
            text_init(c.byref(text))
            assert get_text(session, c.byref(text)) == 0
            return tuple((value or b"").decode("utf-8") for value in
                         [text.full_text, text.committed_text, text.tentative_text])

        def feed_pcm(session, pcm):
            pcm = np.ascontiguousarray(pcm, dtype=np.float32)
            update = Update()
            update_init(c.byref(update))
            start = time.perf_counter()
            status = feed(session, pcm.ctypes.data_as(c.POINTER(c.c_float)), len(pcm), c.byref(update))
            return status, update, (time.perf_counter() - start) * 1000

        assert init_backends(str(dll_dir).encode()) == 0
        params = Load()
        load_init(c.byref(params))
        params.backend = 1  # strict CPU, never Auto
        session = ptr()
        started = time.perf_counter()
        assert open_session(str(args.model.resolve()).encode(), c.byref(params), None, c.byref(session)) == 0
        assert session.value
        load_ms = (time.perf_counter() - started) * 1000
        result = {"version": version, "dll_sha256": digest(dll_dir / "transcribe.dll"),
                  "model_sha256": digest(args.model), "audio_sha256": digest(args.audio),
                  "load_ms": load_ms,
                  "backend": backend(get_model(session)).decode(),
                  "input_seconds": len(audio) / 16000, "events": []}
        assert result["backend"] == "CPU"
        try:
            set_abort(session, abort, None)
            assert begin(session, None, None) == 0
            previous = ""
            seam_mismatches = 0
            for offset in range(0, len(audio), 8000):
                status, update, milliseconds = feed_pcm(session, audio[offset:offset + 8000])
                assert status == 0, (offset, status)
                full, committed, tentative = snapshot(session)
                assert committed.startswith(previous), "Committed prefix rewrote"
                previous = committed
                seam_mismatches += int(committed + tentative != full)
                if update.committed_changed or update.tentative_changed:
                    event = {"input_ms": update.input_received_ms, "revision": update.revision,
                             "committed_chars": len(committed), "tentative_chars": len(tentative),
                             "feed_ms": milliseconds, "seam_matches_full": committed + tentative == full}
                    result["events"].append(event)
                    print("partial", json.dumps(event), flush=True)
            update = Update()
            update_init(c.byref(update))
            started = time.perf_counter()
            assert finalize(session, c.byref(update)) == 0 and update.is_final and state(session) == 2
            full, committed, tentative = snapshot(session)
            result.update(finalize_ms=(time.perf_counter() - started) * 1000,
                          final_raw=full, final_committed=committed, final_tentative=tentative,
                          seam_mismatches=seam_mismatches, final_seam_matches_full=committed + tentative == full)
            reset(session)
            assert state(session) == 0 and snapshot(session) == ("", "", "")
            assert begin(session, None, None) == 0
            abort_flag = True
            status, _, milliseconds = feed_pcm(session, audio[:16000])
            result["abort"] = {"status": status, "state": state(session),
                               "was_aborted": bool(was_aborted(session)), "feed_ms": milliseconds}
            assert status == 13 and state(session) == 3 and was_aborted(session)
            reset(session)
            abort_flag = False
            assert begin(session, None, None) == 0
            assert feed_pcm(session, np.zeros(16000, np.float32))[0] == 0
            update = Update()
            update_init(c.byref(update))
            assert finalize(session, c.byref(update)) == 0
            result["after_cancel_silence"] = {"state": state(session), "text_empty": snapshot(session) == ("", "", "")}
            assert result["after_cancel_silence"]["text_empty"]
            with args.output.open("x", encoding="utf-8") as output:
                json.dump(result, output, ensure_ascii=False, indent=2)
            print("Probe completed; private transcript written only to requested output", flush=True)
        finally:
            close(session)
            print("session/model freed", flush=True)


if __name__ == "__main__":
    main()
