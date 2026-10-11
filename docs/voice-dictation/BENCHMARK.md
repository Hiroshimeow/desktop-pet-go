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

## G04 released controls/native mechanisms — 2026-10-11 ~18:25–18:40 JST

These are bounded API/capture probes, **not G06/G07/G10 acceptance**. Revisions/licenses/limits in G04_RELEASE_AUDIT.md.

### Handy existing instance: start → actual callback → cancel

Verified idle Recording overlay hidden before test; existing Handy29968 preserved, no process takeover. Two iterations of `handy.exe --toggle-transcription` then `handy.exe --cancel`, each child timeout10s; bounded log-condition waits8s ready/5s cancel, not production polling. No stop/final decode/paste/LLM. Actual mic callbacks can include ambient audio in RAM; cancel discards it, no recordings/history files created or sent.

| Trial | Toggle child ms / exit | Observed ready ms from command start | Cancel child ms / exit | Native log evidence |
|---|---|---:|---|---|
| 1 |79.8 /0 |214.1 |139.2 /0 |start completed117.634ms; real mic ready; cancellation completed |
| 2 |88.2 /0 |235.2 |138.9 /0 |start completed91.088ms; real mic ready; cancellation completed |

Both stdout/stderr empty: CLI has no runtime result/live acknowledgement. Readiness observed through existing debug log, not public protocol. Small samples, polling/scheduler overhead; no p95/product latency claim. Overlay visible at readiness; still visible when cancellation log first arrived (async hide), later hidden verified. Foreground HWND unchanged; clipboard sequence unchanged2047; settings SHA unchanged; history row count5 unchanged, existing6 recording files unchanged. No transcript/log contents copied except own technical probe markers. Settings remain user WhisperMedium/Auto, so GUI inference CPU policy/live Nemotron not measured.

### Shipped native public C API (not hidden worker)

Handy DLL `transcribe_version=0.2.4`, runtime commit `unknown`, SHA256 `765e4723dba20404b7ac7108998249599539cbd4ce93f2245b4ec6b2ff6a9a2a`. Matching crate0.2.4 checksums verified against released Cargo.lock. Research probe uses exact public C API, rejects wrong version/ABI layout, serial model/session owner, strictCPU backend, copies borrowed UTF-8. Backend loader enumerates Intel Vulkan but model explicitly logs/returns **CPU strict**; no Vulkan inference in probe.

One synthetic2.784s WAV repeated4 times with400ms zeros between →12.736s PCM; not independent/human corpus. Feed500ms audio blocks as fast as API returns (not real-time microphone replay). Successful initial probe: load1266.217ms; **8 changed live snapshots before finalize**, input cursors1500/2500/4500/6000/8000/9000/11500/12500ms, monotonic revisions1–8/committed prefix. Nonempty tentative never observed. Feed calls producing changes296.515–438.700ms, finalize295.443ms. One fast-fed run cannot establish release-to-final/real-time latency.

Final raw/committed matched, tentative empty; **VI ending badly misrecognized** versus prior batch—does not pass accuracy. No preview seam mismatch in this fixture; source API warns divergence possible, not globally proven invariant. Already-set abort flag returned13/FAILED/was_aborted in0.0618ms; **not** in-flight encoder cancellation latency. Reset clears all text/state, next silence stream finalized empty. Session/model freed in finally, child exit0.

First temporary attempt exited1 due Python cp1252 console UTF-8 print (after successful stream/final, teardown ran); rerun with `-X utf8` passed. Committed reproducibility script `probes/transcribe_release_probe.py` avoids transcript stdout; output JSON stays private/temp, retains upstream struct license notice. First reproducibility rerun exit0 produced same8 updates and silence/abort success; its initial load timing incorrectly included hashing (2413.89ms) and is **not used**. Timer corrected before final rerun: exit0, load **1756.996ms**, finalize **424.929ms**,8 updates, abort13/FAILED in0.0908ms, next silence empty,0 observed seam mismatches. These isolated API timings remain non-acceptance spikes. No audio/model/DLL/private JSON committed.

Reproduce with existing local paths (output must be a **new** private file), launch using Python subprocess `timeout=120`:

```
cd voice-sidecar
uv run --no-sync python -X utf8 ../docs/voice-dictation/probes/transcribe_release_probe.py --dll-dir <existing Handy directory> --model <existing pinned Nemotron GGUF> --audio <short synthetic 16kHz mono PCM16 WAV> --output <new private temp JSON>
```

### OpenWhispr released artifact/native helper

No installed/running app found in standard local/roaming/program directories. Downloaded portable v1.10.2 archive, verified GitHub SHA256/size, extracted with existing7-Zip without executing installer/Electron app. Packaged version1.10.2; seven relevant JS files equal exact released commit `1c123e8739f87c0a72d0b252bc9cff14e684b7f4` after CRLF normalization. No registration, cloud defaults, full app or external HTTP native decode run.

Actual shipped `windows-fast-paste.exe` SHA256 `bf400c7ca860e55df9c16692cca79d4be26232c353af055025f7cc606cdac046`:
- `--capabilities` exit0: `paste-v1 selection-copy-v1 target-identity-v1 focus-restore-v1`.
- `--detect-only` exit0: reports current foreground HWND/class/exe and terminal=false.
- `--restore-window 1 --detect-only` exit0: invalid HWND still reports current foreground, confirms released **fail-open** fallback described by C source. No copy/paste/chord injected; clipboard sequence/foreground unchanged for all probes.

Deterministic released source `node --test <release>/test/helpers/{cliBridgeTranscribe,windowsPasteTarget}.test.js`: exit0, **13 PASS**,135.4741ms; fake model/path/handle tests, no native ASR/paste. `parakeetWsResult.test.js`: exit0, **11 PASS**,123.5742ms, real parser/pure fixture messages. Combined run including clipboardRestore initially exit1:13pass/1 suite failure. ClipboardRestore alone reproducibly exit1 at debugLogger119 `app.isPackaged`; test's Electron fake supplies clipboard/systemPreferences but omits app, non-Windows short-circuit hid it. Upstream unchanged; no suite PASS claimed. Final combined rerun of only API/target/parser suites: **24 PASS/0 fail/0 skip**,229.3142ms. Node22.22.2. No product regression caused by this upstream harness failure.

## Microphone/GUI status

`uv run --no-sync python voice_sidecar.py --list-devices`: exit 0; native Intel microphone array available (MME, DirectSound, WASAPI), default MME input index 1, speakers default index 3. Enumeration is **not capture/recognition evidence**. Subsequent G04 Handy toggle→real-callback→cancel is verified above; **stop/final/paste/live GUI/hold remains pending**. Pet click/drag/voice/TTS, dictation target tests, focus/elevation/clipboard races, release-to-final percentiles and human VI/EN corpus still pending.

Optional LAN GPU/cloud experiments not run or authorized; no cloud audio upload. Installed Handy Auto device defaults are not adopted as Desktop Pet CPU policy.
