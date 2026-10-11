# Upstream parity/reuse investigation

Status: **preliminary** (do not claim full mapping). Research input repositories below; pin SHA, inspect actual codepaths and run native UX before implementation.

| Reference | URL / initial observed value | Proposed role | What remains to verify |
|---|---|---|---|
| OpenSuperWhisper upstream | https://github.com/Starmel/OpenSuperWhisper — native macOS, MIT | OSW-level **UX/behavior standard** | pin SHA; inspect hotkeys, overlay, history, insertion, glossary, model retention; confirm live partials really implemented |
| Handy | https://github.com/cjpais/Handy — cross-platform, MIT, Windows release, local ASR; `--toggle-transcription` CLI | **First external reuse candidate** | G6 install/CLI, actual hold semantics, partial events, clipboard/target focus, supported embedding |
| OpenWhispr | https://github.com/OpenWhispr/openwhispr — cross-platform, MIT, local/cloud | Secondary replacement/IPC option | inspect actual control API, resource cost, config and input insertion |
| OSW-Windows | https://github.com/alpacarodeo/OpenSuperWhisper-Windows — MIT Windows community port | Port patterns if proven | SHA/code quality, warm model, latency, failure handling; not equal to macOS upstream by assumption |
| Whisper Local | https://github.com/drajb/whisper-local — MIT | streaming/live UX comparison | implementation of pre-roll, partials, model warmup and Windows paste |
| TakanariShimbo OSW | https://github.com/TakanariShimbo/open-super-whisper — MIT, Python/PyQt | **Not primary**: OpenAI API transcription only | G6 code at 711fb4f confirmed OpenAI API call and clipboard-only rather than auto-paste; GUI framework import passed |
| local-live-caption-engine | G6 `E:\python_project\local-live-caption-engine` | Tested segmenter/stabilizer algorithm donor | pin current revision/license; benchmark with Vietnamese, stable partial, overlap, CPU |
| Desktop Pet sidecar | `voice-sidecar/voice_sidecar.py` | Existing runtime owner | running quality/latency, protocol change and resident model behavior |

## Required source mapping before implementation

Trace and cite exact upstream **source file, function, commit, license** for (1) global toggle/hold, (2) capture/VAD/pre-roll, (3) model lifecycle, (4) partial/stable/final, (5) cancellation/retry, (6) original target identity + safe paste, (7) clipboard restoration, (8) overlay/DPI, (9) history/retention, (10) multilingual glossary. Rate each `PRESERVE`, `REUSE_EXTERNAL`, `PORT`, `GAP`, `OUT_OF_SCOPE`; unexplored is `UNVERIFIED`, not PASS.

**Decision:** verify Handy end-to-end first; never treat its CLI toggle as equivalent to fully embeddable streaming API. If parity unavailable, port only proven mechanisms into the existing Desktop Pet voice stack, retain attribution when code is reused. No new independent dictation product.
