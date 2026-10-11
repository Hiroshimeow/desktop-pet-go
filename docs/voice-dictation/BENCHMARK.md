# Windows measurements — preliminary, not acceptance

2026-10-11, G6 ThinkBook model `21LE`, Intel Core Ultra 7 155H, 16 cores/22 logical processors, 33,945,935,872 bytes RAM. Windows native processes, no cloud/GPU inference in trials below. Existing installed Handy `0.9.8` (`%LOCALAPPDATA%/Handy/handy.exe`). Canonical Go `1.26.1`, Python `3.11.15`, uv `0.11.8`.

**Scope warning:** one existing local synthetic VI WAV, `identity_bet.wav`, from preserved acceptance snapshot `.voice/acceptance/candidates/`, mono int16 16kHz, 44,544 samples / 2.784 seconds. No human corpus ground truth established. No WER/CER/terminology acceptance claim. No raw audio committed or uploaded. No release-to-final/hotkey/GUI claim from batch timings. Small trial counts are spikes, not production p95 evidence.

## Installed Handy CLI (WINDOWS-RUN-VERIFIED)

Invoked via canonical `uv run --no-sync python` subprocess with 15s metadata / 180s inference timeout and captured stdout/stderr.

- `handy.exe --help`: exit 0. Exposes toggle, cancel, file transcription, model/device selection, repeat and JSON; no CLI live-event subscription listed.
- `handy.exe --list-models --json`: exit 0. 69 catalog models seeded. Whisper Medium, Nemotron 3.5 and Canary downloaded according to CLI. `%APPDATA%/com.pais.handy/models` empty because shared HF cache also searched; empty directory must not be mistaken for missing models.
- `handy.exe --list-devices`: exit 0, index 0 Intel Arc Vulkan, index 1 Intel Ultra 7 CPU. Installed settings report Auto accelerator, selected Whisper Medium, postprocess disabled, auto-submit false. No settings/secrets dumped or changed.

Commands (same WAV, explicit **CPU index 1**, repeat 3, JSON):

```
handy.exe --transcribe-file <local synthetic WAV> --model handy-computer/whisper-medium-gguf/whisper-medium-Q8_0.gguf --device-index 1 --repeat 3 --json
handy.exe --transcribe-file <local synthetic WAV> --model handy-computer/nemotron-3.5-asr-streaming-0.6b-gguf/nemotron-3.5-asr-streaming-0.6b-Q8_0.gguf --device-index 1 --repeat 3 --json
```

| Model | Exit | CLI load ms | Decode trials ms | Process wall s | Bound backend |
|---|---|---:|---|---:|---|
| Whisper Medium Q8_0 | 0 | 869 | 16857, 18065, 17271 | 53.346 | CPU |
| Nemotron 3.5 streaming Q8_0 (batch CLI trial) | 0 | 1278 | 645, 565, 605 | 3.432 | CPU |

Whisper preserves final question in this sample; Nemotron changes ending to a statement/wrong final word. Both render synthetic wake-name as a Vietnamese spelling rather than product identifier. Faster Nemotron result is **not** accuracy superiority. CLI RTF field is audio/decode speed ratio (4.927 for fastest Nemotron), not conventional decode/audio cost; retain raw ms to avoid confusion. Nemotron streaming capability reported by registry/native load, but **live streaming not measured yet**.

### Internal worker incompatibility (WINDOWS-RUN-VERIFIED failure)

Installed `handy.exe` SHA256: `9a1d2246b64322cb334960a5b3830b0d125070735c355648c498076ec7c0a0ff`. Probe `handy.exe --transcribe-worker --cpu-only` exited **2**, stderr `error: unexpected argument '--transcribe-worker' found`; framed reader raised `EOFError: worker EOF`. Child cleaned up, no persistent worker remains from probe. Default source `48a3d8e...` implements the flag, but exact installed release v0.9.8 source is `14f6f0d31cb22a4268acfbddba3052820fd0a06e`. This demonstrates a version/API incompatibility, not working streaming integration. Source-level stream capabilities and fast batch timing do not establish a supported resident live API.

## Existing faster-whisper base/int8 CPU (WINDOWS-RUN-VERIFIED batch spike)

Canonical `.voice` was absent initially. Read-only source assets from preserved clean acceptance snapshot were SHA256-verified against all eight `voice_sidecar.EXPECTED_SHA256` entries before copying to ignored canonical `.voice/models`. Existing destinations would be checked, never blindly overwritten. All eight verified; `git check-ignore .voice/models/faster-whisper-base/model.bin` confirms ignored. No network model download.

`WhisperModel(str(STT_DIR), device='cpu', compute_type='int8', cpu_threads=0, local_files_only=True)`; production transcribe parameters beam_size=1, best_of=1, temperature=0, condition_on_previous_text=False, vad_filter=False. Same 2.784s WAV. Six sequential decodes, lazy segments fully consumed. Process working set obtained using native GetProcessMemoryInfo; CPU time via `time.process_time`.

- Load: **639.021ms** (file cache state uncontrolled; not a disk-cold claim).
- Decode ms: **1859.250, 2319.376, 2149.504, 2922.203, 2896.832, 2417.060**.
- Warm trials (exclude first): p50 **2417.060ms**, linearly interpolated p95 **2917.129ms**, n=5 only.
- CPU ms per trial: 7234.375, 8921.875, 8406.250, 11046.875, 11312.500, 9578.125 (sum over threads, not wall percentage).
- Inference working set ~227.5–231.8MiB; final sampled 233.4MiB. No separate idle CPU sample yet.
- All trials detected vi and same question text; product-name spelling still imperfect.

## Existing local-live-caption deterministic comparator

`cd E:/python_project/local-live-caption-engine && .venv/Scripts/python.exe -m pytest -q -p no:cacheprovider`: exit 0, **27 tests in 1.08s**. Did not sync/mutate its dependencies/source; untracked user tools untouched.

Additional actual-code probes from canonical Python (exit 0):

- Stabilizer updates `hello world` three times then `totally unrelated`: result keeps `stable='hello world'` while `full='totally unrelated'` and unstable is unrelated. Stable+unstable no longer represents current full hypothesis.
- Stitcher appends `test`, then independent `today works`: output **`test oday works`**, trims one shared character despite no actual audio overlap.

These expose missing invariant coverage despite its green suite. Do not copy stabilizer/stitcher unchanged into dictation or claim proven VI accuracy. Segmenter pre-roll/current-frame-once and scheduler latest-partial replacement remain useful candidate techniques.

## Microphone/GUI status

`uv run --no-sync python voice_sidecar.py --list-devices`: exit 0; native Intel microphone array available (MME, DirectSound, WASAPI), default MME input index 1, speakers default index 3. Enumeration is **not capture/recognition evidence**. Real Handy toggle/paste, pet click/drag/voice/TTS, dictation target tests, focus/elevation/clipboard races, release-to-final percentiles and human VI/EN corpus still pending.

Optional LAN GPU/cloud experiments not run or authorized; no cloud audio upload. Installed Handy Auto device defaults are not adopted as Desktop Pet CPU policy.
