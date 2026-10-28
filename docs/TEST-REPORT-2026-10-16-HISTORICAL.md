> Historical 16 October source/test report. Its final-source wording refers only to that date. Use [the 28 October report](TEST-REPORT.md) for this bundle's source and current limits.

# MOO2-SGC launcher 0.4.8 candidate: engineering verification

Date: 2026-10-16 UTC. Status: **unreleased source candidate**, not a signed installer, multiplayer release, or gameplay certification.

## Summary

The supplied 0.4.7 code has been reviewed and hardened. The changes close reproducible integrity and runtime-reporting problems while preserving the baseline/Community separation. Final-source checks pass within the tested scope below. No GitHub, publication, signing, or external service action was taken. No owned game or runtime bytes are in the deliverable.

## Implemented changes

1. **Do not let a local receipt certify its own contents.** The supported 1.50.26 patch now has a compiled 128-file hash inventory. Immutable scripts/code/rulesets must match that inventory, selected ENABLE.CFG must equal the resolver output, the stored selection fingerprint must match the prepared profile, and legacy archive provenance no longer skips installed-base validation. The schema-3 ORIONCD.INI harness contract is checked explicitly.
2. **Make effective configuration identity complete.** Added editable CFGs and additional data files contribute to the fingerprint. Unsupported extra protected code/config cannot be blessed by adding it to the local manifest. A conservative scanner follows nested includes, quoted script lists, known language macros and caret escapes; it refuses custom use of save/audio/generated files whose contents are otherwise excluded from identity. It is not a complete MOO2 rules evaluator and may reject unusual custom configurations conservatively.
3. **Distinguish gameplay outputs from inputs.** EXTRACT.CFG and ORION2.LOG do not block verification or change compatibility after a game run. 150.SET and NEW150.SET are personal state and are retained during same-engine repair. Using excluded outputs as custom configuration inputs is rejected.
4. **Distinguish process start, process exit and actual success.** Runtime failures are written to the launch/diagnostic record. CLI diagnostic/prepare-play waits for completion and returns failure when the runtime exits unsuccessfully. Normal process exit still does not certify game rendering, connection or multiplayer. Start-failure logging was reviewed; the dynamic tests explicitly cover exit 0 and exit 47.
5. **Fix asynchronous duplicate selection.** The UI does not switch to a missing or failed duplicate. It selects a successful copy after save completion, ignores older save results, and preserves a newer navigation choice.
6. **Make source-archive builds reliable.** The development cross-build disables VCS auto-stamping. Version 0.4.8 is numeric for the existing parser, not a stability designation.

## Final verification results

| Check | Result | Actual scope |
|---|---:|---|
| Go race suite, all supplied fixtures | 257 passed; 0 failed; 0 test skips | 136 top-level tests plus subtests; 5 tested packages passed. shared/buildconfig has no test files. |
| go vet | Passed | Final source, all packages. |
| JavaScript syntax | Passed | manager/web/app.js. |
| Duplicate-profile regression | Passed | Deterministic Node/VM event-flow test: rejected request, delayed save, successful/failed save, stale result, newer navigation and missing saved profile. Not a browser run. |
| Publishing safeguards | 10 passed | Mocked service only, no account/remote calls. |
| Portable/owned-file acceptance | 50 passed | Current real Linux launcher; baseline, Steam and CD lineage, optional patch, exact file/kernel hashes, source preservation, repair, rollback, synthetic saves and locks. DOSBox process double, no game execution. |
| Network diagnostic and selection | 24 passed | Current real Linux launcher/API/CLI with an explicitly labelled process double. No DNS/UDP/Dopefish reachability claim. |
| Cross-compilation | 4 targets passed | Windows amd64, Linux amd64, macOS amd64 and macOS arm64 launchers. Outputs are unsigned and not included. |

Source-built candidate binaries were used for the final 50/24 acceptance runs. Local loopback HTTP/HTTPS fixtures are allowed and passed. They are not external network tests. Original owned archives were compared before/after in portable acceptance and remained unchanged. Save tests check bytes, not whether MOO2 can load them.

## Evidence and chronology

- `evidence/recovery-settings-final-all-fixtures-go.jsonl`: authoritative final 257-pass race run after the real-runtime settings fix. `recovery-final-all-fixtures-go.jsonl` records the prior 256-pass run. `recovery-final-go.jsonl` is the immediately preceding 247-pass/2-skip run; the skips were then resolved using supplied private fixtures.
- `evidence/recovery-settings-portable.json` and `recovery-settings-network.json`: final compiled Linux acceptance after the settings fix. Earlier recovery-portable/network files precede that fix.
- `evidence/recovery-publishing.log`, `recovery-ui-regressions.log`, and `recovery-validation-summary.json`: final static/build/unit evidence and binary hashes.
- `evidence/review-final-go.jsonl`: recovered earlier 244-pass/2-skip run. Three source files changed after that run, so it is not treated as final-source proof.
- `evidence/regressions-before.log` and `ui-regressions-before.log`: reproduced pre-fix failures; their after logs show targeted repair.
- `evidence/process-failure-before.json` / `process-failure-after.json`: the 0.4.7 CLI incorrectly returned 0 after runtime exit 47; the candidate returns 1 and records exit code 47.
- Historical `QA-SUMMARY.json`, `FINAL-PACKAGING.json` and the original test report belong to the supplied 0.4.7 release. They are not candidate results. The historical report is retained as `TEST-REPORT-0.4.7-HISTORICAL.md`; old signed release assets and old QA bundles are excluded from this source archive.
- The initial rerun's default Go cache path was read-only; using a cache inside the permitted workspace resolved that local setup issue. This was not a code failure.

## Blocked, untested and deliberately not done

- **Direct Chromium UI acceptance was blocked:** the recovered launch log records `socket() failed: Operation not permitted`. It failed before UI assertions. The denied operation was not retried or bypassed. Node/VM event tests do not replace native browser acceptance.
- **Windows and macOS were compiled, not executed.** No native installers, dialogs, browser lifecycle, runtime launch or native game session on those platforms was certified.
- **No live Dopefish, UDP/IPX or multi-machine multiplayer test.** Configuration generation and process lifecycle are separate from transport reachability and actual game turns.
- **No actual DOSBox or MOO2 execution by the original 50/24 automated launcher suites.** The separately scoped real-runtime addendum below closes a narrow Linux title-screen startup path only; it does not turn those process-double checks into real gameplay tests.
- **No new signed distribution/upgrade acceptance.** Existing 0.4.7 signatures do not authenticate 0.4.8 source or binaries. Installer/release workflows require a separately built and signed new release and were not executed. Authenticode and macOS notarization are absent.
- **No PRSL/new Chat integration or new gameplay presets.** These remain independent; selecting a launcher mod profile does not configure every in-game New Game option.

## Reproduction

From the source root, use Go 1.23+:

```
cd manager
go test -race -count=1 ./...
go vet ./...
cd ..
node --check manager/web/app.js
node tests/ui_regression.js
python3 -m unittest discover -s tests -p 'test_publish*.py' -v
sh manager/build.sh
```

For the complete private-fixture race run, set MOO2_COMMUNITY_FIXTURE to a prepared Community generation containing manifest.json and game/, MOO2_HARNESS_FIXTURE to the supplied Windows runtime directory (read/verify only), and MOO2_TEST_ASSETS to a private fixture directory. The latter must contain the original supplied Steam ZIP named `Master of Orion 2 - steam - v1_40b23.zip` and the recognized official 1.31 patch archive named `_moo2v131_patch.zip`. The latter name is a fixture alias for the already-recovered MOO2-1.31.en.zip, not a new download. Source tests verify the relevant actual file contents. Without fixtures the relevant tests explicitly skip, not pass.

Use `tests/portable_acceptance.py --help` and `tests/network_check_acceptance.py --help` for compiled Linux acceptance. They use only copies/temporary directories and a labelled process double. Do not point runtime gameplay tests at an irreplaceable live installation. Do not follow old deployment instructions for this candidate.

## Next acceptance gate

Review the source patch, then test the candidate separately on Windows/macOS with the owned runtime and real game. Cover UI interruption/repeated actions, upgrades/repair/rollback, save loadability, both baseline and Community, process failures, native browser behavior, and real multiplayer. Only then decide whether to build and sign a new release. Applying the source patch to an older checkout leaves historical release assets untouched; they must never be relabelled or republished as this candidate.


## Separately scoped real-runtime integration addendum

**2026-10-16: PASS for the exact final Linux launcher -> real DOSBox Staging 0.83.0 -> MOO2 1.50.26 title-screen startup path.** This is a separate real-emulator integration observation; the original 50 portable and 24 diagnostic checks above remain labelled process-double tests.

The actual manager imported the supplied owned base/patch, prepared Community standard (engine 1.50.26, core 150, no extra mods, standalone/network none), verified the fixed kernel, selected the runtime and launched the game. The emulator image visibly shows Version 1.50.26. The process exited 0; post-run verification passed **541 entries**, with no bad files or reported user changes, and the effective fingerprint was identical at preparation, launch and post-run verification.

Final Linux launcher SHA-256: `07f63bab96679a357121f9a25224384c915190b691543a460b0c40bce32624d6`.

Engine SHA-256: `2db296e052419250d21866f7c23ac2978a33b3c05b451a9516f06599b91c3f5c`.

RKERNEL SHA-256: `18e8781f8ce64516e60b2947b487e8999f7d940b25af971359c7e7dea0fe9d97`.

Stable effective fingerprint: `1dc35679eeee79ee7f462eb3e51db1524a312d4df3bcbce3422b6d4328f24336`.

Evidence: `evidence/recovery-real-launcher-smoke.json` and `evidence/recovery-real-launcher-title.png` (image SHA-256 `60d25b3ab4d90058e86b679df4452ddac07c2a0861899e0ac73a3139d29d68c3`). The screenshot was visually inspected, not inferred from normal exit. The originating runtime report is titled `FINAL-LAUNCHER-RUNTIME-2026-10-16.md` in the separate runtime evidence deliverable.

### Additional reliability defect found and fixed

The first real startup used the preceding Linux build `dfcdb06b23ff6158a90e4b12a34f41f7c8e186a1afc1f354e21225fb55aa2c3b`. Engine/kernel identity and 540-file verification passed, but the effective fingerprint changed because the game generated **150.SET**, a normal personal-settings file documented by the supplied 1.50.26 manual. The launcher previously recognized NEW150.SET but omitted 150.SET; this also meant the latter was not retained by same-engine repair. EXTRACT.CFG and ORION2.LOG were already handled correctly.

The fix adds only the exact root-level 150.SET name to personal-state handling, preserving it and excluding its normal settings bytes from mod compatibility. It does not exempt arbitrary SET files. The custom-config scanner also rejects references to 150.SET as an excluded input, retaining the integrity boundary. The synthetic regression fails before the fix and passes after it; the final real smoke additionally demonstrates stable before/after identity. The repeat prepared a generation retaining the settings file, hence 541 checked entries rather than the earlier 540.

See `evidence/recovery-real-launcher-pre-fix.json`, `recovery-150-settings-before.log`, and `recovery-150-settings-fix.json`. No 150.SET or save payload is packaged. The refreshed 257-test race run, 50/24 process-double suites, vet and four cross-builds all cover the final source. No source-code changes followed the final build.

### Boundaries remain explicit

The real runtime is invoked through a test-only host wrapper that forwards the manager's arguments and uses dummy video/audio, software rendering, no mouse capture, fixed CPU cycles and scripted native SDL input/capture. Engine, kernel and manager-generated game configuration are not patched; success is not simulated. This is adapted headless Linux startup, not native desktop rendering/audio or browser UI acceptance.

This smoke reaches the title screen, not a played turn or save/reload cycle, and its profile has no network service. It does not certify Staging multiplayer, Dopefish, LAN/internet reliability, Windows/macOS, PRSL or tactical gameplay. A separate Classic 0.74-3 two-client native-turn/save/reload result belongs to the runtime workstream and was **not driven by this final launcher**; it must not be attributed to this launcher smoke or added to the 50/24 counts. Personal New Game choices are separate from the mod fingerprint and still require explicit in-game acceptance.
