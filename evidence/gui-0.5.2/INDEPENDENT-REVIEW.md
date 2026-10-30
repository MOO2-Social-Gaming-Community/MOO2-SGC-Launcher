# Independent stability/package review — 0.5.2

**PASS for a private review build; not approval for a stable public link.**

## Verified final scope
- 367 Go race tests/subtests passed; zero failures/skips; seven tested packages. `go-final.jsonl`, `vet-final.log`.
- Seven Windows test-bearing packages crosscompiled, not executed. The main package was recompiled after the final pre-start history fix. buildconfig has no tests.
- Four deterministic Node regression suites, two JS syntax checks, 22 Python packaging/publishing safeguards passed. No remote write occurred.
- Independently reran final Linux outer/manager process checks: 11 headless native smoke, 30 desktop lifecycle (browser-opener recorder), eight two-build isolation checks. Counts describe separate suites, not unique or Windows gameplay cases.
- Independent Ed25519 manifest and package signatures, all five inner-package member hashes/sizes, exact embedded bytes, clean four-file output and 150-file current source index verified for final Windows/Linux bundles.
- Windows executable is x64 PE GUI subsystem2 with no Authenticode certificate.

Windows EXE SHA-256: `e37a4a6d17d41f98daf974c27d38de13127052b15fe30e62b85ba5d106069315`.
Source-index SHA-256: `8e7602eafd62f8a6c67eef9443d1aaba7f51a1e13b6d07cf8bd34e5565692033`.

## Findings repaired and reviewed
1. Source-only pushes previously invoked missing production-release inputs and never built the outer desktop launcher. The replacement workflow tests source, builds a Windows review executable and uploads an Actions artifact with read-only repository permission; no Release/tag/latest promotion.
2. Rebuilding the same development version/revision with a fresh key reproduced manifest-equivocation failure; old live sessions were identified only by version/root. Full public-trust fingerprint ownership and hash-suffixed roots now isolate different builds before session reuse. Actual two-binary same-version/revision regression passed, retaining original ownership, generation, session and synthetic save.
3. Identical rapid failed jobs could disappear after UI cleared the previous error. Per-operation IDs now distinguish completions.
4. Failed state polls could permit stale preview results to reenable controls. Connection loss invalidates preview generations and gates mutations, including actions waiting on selection persistence; deterministic repeated/interrupted-flow regressions pass.
5. Preference-save failure after pending launch-history write, but before process Start, could leave falsely ambiguous history. A deterministic negative process witness reproduced it; the narrow fix records only proven no-start and preserves journal-write failure as an error. Retry process-double test passes. Unknown/surviving games remain protected, not silently unlocked.

## Evidence integrity
Private Steam, patch, harness, official runtime and genuine manual legacy fixtures were restored separately. All input hashes and pinned inventories matched; real manager imports/preparations verified Steam and genuine legacy Community copies without running game/emulator. Final aggregate has no fixture skips. Earlier/interim results are retained as such, not relabelled final.

The only source change after final Go compilation was packaging/build_review.py's optional separate public evidence-retention directory. Python safeguards and real final artifact signature/source verification were rerun on that change. No Go or JavaScript changed after their final checks.

## Remaining release gates
Native Windows workflow execution and actual visible browser/picker/game/audio/save-loadability testing are unperformed here. No prior Chromium socket denial was retried/bypassed. Production signing, signed-manifest expiry/durable availability, supported data-preserving updates and conservative ambiguous-game crash recovery remain explicit stable-release gates. Crosscompile, Linux process doubles and internal development signatures do not close them. No GitHub publication or credential use occurred.
