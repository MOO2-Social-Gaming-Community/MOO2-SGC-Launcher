# 0.5.1 — Private Windows-first GUI candidate (2026-10-29)

- Keep the delivered 0.5.0 and published 0.4.8 checkpoints unchanged. The new candidate uses a separate per-user test root and fresh setup.
- Add official-store acquisition guidance and return-to-import controls; no game redistribution or unsupported installer execution.
- Show bounded download progress and cancel blocked network requests safely, preserving verified caches and prepared games; retry does not inherit cancellation.
- Add saved per-profile display, audio mute and background behavior settings without changing compatibility fingerprints, game rules, or emulator hardware.
- Keep native Windows, rendered browser, actual gameplay/save, signing and publication gates explicit. Linux/API tests and cross-compiles do not certify Windows.

# 0.5.0 — Private GUI candidate (2026-10-28)

- App-style single-file entry point with offline signed launcher embedding, automatic graphical startup, isolated per-user data, repeated-click session reuse, and visible progress/retry/recovery/exit.
- Graphical source/runtime selection and verified preparation; Community 1.50.26 standard rules suggested for new GUI users, legacy 1.40b23 retained as a separate explicit option, existing profiles preserved.
- Windows GUI-subsystem binaries and hidden console helpers; no terminal, Go install, CMD wrapper or config editing in the player workflow.
- Conservative interruption recovery preserves live/unknown launcher and game states; no changes to original installations, saves, or shortcuts.
- Normal initial browser deferral is separate from explicit headless policy, including signed update/restart handoffs.
- No PRSL/new Chat readiness claim, new balance preset, game-data bundle, production signing, or publication. Native desktop/gameplay acceptance remains separate from Linux/API and cross-build evidence.

The user reports publishing 0.4.8; that baseline is unchanged. Entries below describe earlier work.

# 0.4.8 candidate — unreleased, 2026-10-16

- Pin the supported patch inventory and generated selection independently of writable workspace manifests; validate legacy base identity and generated ORIONCD.INI.
- Fingerprint newly added editable CFGs and extra data, and conservatively scan nested configuration references to excluded personal/audio/generated files.
- Exclude EXTRACT.CFG and ORION2.LOG runtime outputs from compatibility identity; preserve 150.SET and NEW150.SET as personal state.
- Save process start/exit failures in diagnostics and propagate runtime failure to CLI exit status without claiming connection success.
- Make duplicate-profile selection wait for completed save and preserve newer navigation.
- Add integrity, reference-scanner, process and deterministic UI regressions. Compile without VCS auto-stamping for source-archive builds.
- Real launcher-to-Staging startup revealed 150.SET personal settings were incorrectly fingerprinted as extra data; classify and retain that exact settings filename without exempting arbitrary SET files.
- Source-only handoff. No release signing/publication, private game redistribution or multiplayer certification.

The entries below are historical shipped/supplied version notes, not new candidate validation claims.

# 0.4.7

- Remember the selected profile in userdata/ui-selection.json across launcher restarts and changing localhost ports. First-run default stays baseline 1.40b23; no automatic game upgrade.
- Historical engines show a real not-applicable Core selection rather than a disabled 1.50 standard label. Community remains independently selectable.
- Show the effective network command, endpoint, selected engine and executable before launch, from the same resolver used to generate config.
- Integrate an explicit Dopefish CONNECT/STATUS diagnostic into the launcher and locked CLI. It uses the verified runtime, mounts no game and records last-network-check.json. Process startup is NOT reported as connection success.
- Bundle CHECK-DOPEFISH.cmd in the portable kit; route it through the signed installer/launcher so existing runtime and process-lock protections apply.
- Clarify setup's previous-version line. Preserve 0.4.6 minimum-version floor, pinned fallback, kernel preflight and source isolation.
- PRSL, new Chat and future SGC Online remain disabled. No balance or native game changes.

# 0.4.6 — version-selection and multiplayer-kernel recovery

- Reject a signed launcher release older than the running setup/launcher before accepting metadata or activating files. A stale GitHub latest endpoint falls through to the pinned v0.4.6 endpoint.
- Re-verify the selected executable and run its version health check before handoff, including reused installations. Print setup version, launcher version, executable paths, and application root.
- Release publication checks GitHub's latest pointer, repairs it for the verified current release when necessary, and never deliberately demotes a newer release.
- Support exact-hash import/normalization of an owned RKERNEL.COM or rkernel.zip without distributing commercial driver bytes.
- Rebuild old 0.4.2 source snapshots using a separately verified local kernel. All 1.40b23/1.50.26 builds must include the canonical driver in the exact executable directory.
- Show per-profile kernel preflight and actual running launcher identity. Block launch on missing/altered kernel even in standalone mode, since players can select Network in-game.
- Record game working directory, kernel path/hash, service, arguments, launcher identity, and runtime in diagnostics and launch logs.
- Preserve baseline, source files, saves, signatures, full-screen/window settings, and Direct/Dopefish services. PRSL, Chat, and future SGC matchmaking remain disabled.

# Changelog

## 0.4.5 — 2026-10-05

- Requires and verifies the exact LAN-fixed `RKERNEL.COM` for 1.40b23 and 1.50.26 before launch, preventing the observed Network Game crash from older managed environments that omitted the runtime kernel. Repair/rebuild restores it from the verified owned baseline without modifying Steam/GOG or the source ZIP.
- Adds selectable network transports: **Direct / LAN**, **moo2.thedopefish.com** (legacy third-party public DOSBox IPX service), and a disabled **MOO2-SGC Online** placeholder for future first-party matchmaking/relay.
- Separates MOO2's in-game Create/Join role from transport selection. With the Dopefish service, every participant connects to `moo2.thedopefish.com` on UDP 213, then creates or joins the named game inside MOO2.
- Migrates old portable source receipts to the complete verified 1.40b23 baseline when available, so existing single-player environments can be safely repaired for networking.

## 0.4.4 — 2026-10-05

- Fixes **Prepare game for play** when the user supplies a recognized owned-game ZIP directly. The source field now content-detects and fingerprints MOO2 archives instead of treating them as the old exact `base.zip` payload format.
- Makes windowed DOSBox Staging explicitly scale the MOO2 image to the available viewport with `viewport = fit` and `integer_scaling = off`, preserving MOO2's 4:3 aspect ratio while removing avoidable internal padding.
- Keeps the portable 1.40b23 baseline, Steam/GOG source isolation, 1.50.26 optional community environment, PRSL separation, and signed update architecture unchanged.

# 0.4.3 — Portable runtime integration

Canonical root C:\Games\MOO2-SGC; manual 1.40b23 baseline import/normalization; exact RKERNEL correction; full local harness runtime verification; original launch/audio settings; transactional root game backups/recovery; optional community profile isolation; same-key update compatibility. See docs/RELEASE-NOTES-0.4.3.md.

# 0.4.2


Fixes the clean Steam import rejection encountered in 0.4.1. Recognizes the verified English Steam DOS 1.40b23 baseline and English CD 1.2 data, without modifying the source installation.

- Steam: 1.40b23 → 1.50.26.
- CD: 1.2 → official 1.31 → 1.40b23 → 1.50.26.
- Separate current, baseline, original CD and legacy profiles.
- Archive import based on known file contents, not ZIP filename/outer hash alone.
- Historical b23 file-transform output matches the supplied clean Steam executable exactly; no XP-era Windows patcher is run.
- Default managed sound setup avoids requiring MT-32 ROMs. Original CD uses an appropriate launch command.
- New schema-2 lineage receipts, compiled file verification, source preservation and same-engine save retention.
- Existing 0.4.1 signing identity and user-data location retained. Public release contains no full game payload or private key.

File/import/build and application tests ran on Linux. Windows/Mac binaries were cross-compiled; Windows game execution and two-client multiplayer remain user acceptance tests. PRSL and new Chat remain disabled. This update does not provide a matchmaking or relay service.

---

# Changelog

## 0.4.1

- Exact user repository wired into signed online bootstrap endpoints.
- Complete GitHub Desktop drop-in tree including reviewed release inputs.
- Automatic verify/upload/readback/publish workflow; repeat runs are safe and non-clobbering.
- A signed copy of Setup in each launcher package enables staged-update restart.
- Per-user Start-menu shortcut and visible Windows double-click errors.
- Explicit stable installation namespace preserves older development installations.
- Known-folder detection, fingerprint-verified local DOS-folder import and one-action preparation.
- Emulated C: data path and matching Sound Blaster IRQ configuration.
- Source-to-binary build-input fingerprint; metadata protected against Git line-ending normalization.
- Three-part version only; existing limitations are not hidden by the version label.

Previous narratives and test evidence are retained under docs/history and manager/evidence. They are not 0.4.1 test results.
