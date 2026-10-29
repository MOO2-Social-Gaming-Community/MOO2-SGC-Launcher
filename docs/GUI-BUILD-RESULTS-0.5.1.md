# 0.5.1 private Windows-first candidate: build and review results

The delivered 0.5.0 source and executable remain unchanged. This increment uses a separate `private-gui-0.5.1` per-user root: it is a fresh setup, not an automatic profile/save or development-signing-trust migration. No GitHub publication was performed.

## Implemented changes

- Fixed official Steam/GOG acquisition links and return-to-discovery action, followed by verified local import. The complete commercial game is **not** downloaded by the launcher; no third-party backup URL or legacy installer is run.
- Throttled download byte/percentage progress, labeled unknown totals, genuine cancellation during blocked HTTP headers/body, partial-file cleanup and retry without inherited cancellation. Failed downloads do not replace verified caches or prepared games.
- Per-profile fullscreen/window sizing, mute audio, background mute and single-player background pause, without changing game rules or compatibility fingerprints. Saved preferences feed the actual launch configuration and arguments.
- Browser session-storage failure fallback, with authenticated reload retained; native and rendered behavior remain separately untested.

## Final checks

| Check | Result |
|---|---|
| Independent Go aggregate race run | **351 test/subtest passes, 0 failures, 3 explicit fixture skips**, seven tested packages |
| Go vet | Pass |
| JavaScript syntax and three Node/VM regression suites | Pass; not rendered-browser tests |
| Publishing safety mocks | 10 pass; no publication attempted |
| Real Linux manager presentation/config/argument/persistence check | Pass using an explicitly labeled process double; no DOSBox or game executed |
| Offline development signatures, all four inner package inventories/hashes/source indexes, exact embedded outer payloads | Independently verified; seven grouped artifact checks |
| Windows outer, manager and helper PE structures | GUI subsystem 2; no Authenticode certificate |
| Compiled Linux outer lifecycle/API | 30 pass, with browser-opener recorder, not a rendered browser |

The three skipped tests need genuine official DOS 1.31/legacy-derived fixture inputs: `TestRealBaselineMatchesSteam`, `TestRealOfficialDownloadContentAuthentication`, and `TestLegacyReceiptKeepsValidFilesAndRejectsForgedBase`. No executable was synthesized and no verifier was weakened to satisfy them. Steam-to-Community, the supplied runtime inventory and the original pinned official runtime archive were available for the final tests. Private fixtures are excluded from source and release artifacts.

## Build identity

- Version: **0.5.1** (numeric, private development candidate).
- Toolchain: existing `go1.27.3 linux/amd64`.
- Indexed build inputs: **142**.
- Source-index SHA-256: `3d6f451426531cc4fc8ea380cd24a0f3731317c393cf029f9517e3f75eac338b`.
- Windows delivery: `MOO2-SGC-0.5.1-Windows.exe`, **15,236,096 bytes**.
- Windows SHA-256: `908b784e9ec8733ba2f282b84c735c285110e7fee74f9325ae9175aa5c27cd7a`.

`SOURCE-CHANGES.patch` is the text-source continuation from the delivered 0.5.0 source. Generated verification evidence and the patch itself are excluded from that diff; the complete source ZIP includes them. `SOURCE-FINGERPRINT.json` indexes code/build/test inputs, not documentation or generated evidence. The signed inner packages embed that exact index.

## Still unverified

Native Windows dialogs, visible game-window behavior, rendering/DPI, audio, real gameplay/save/load and multiplayer remain open. No production OS signing, macOS notarization or rendered-browser certification is claimed. Ubuntu and macOS binaries are internal build checks, not promised one-open consumer apps. See [the Windows acceptance checklist](WINDOWS-FIRST-ACCEPTANCE-0.5.1.md).

Normal players open the EXE; Go, Python, build scripts and source tests are maintainer tooling, not player setup steps. Do not bypass Windows security protections or organizational policy.
