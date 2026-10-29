# MOO2-SGC Launcher — 0.5.1 Windows-first GUI candidate

A friendlier app-style launcher for **Master of Orion II**: on Windows, open one launcher EXE, choose your owned game and community setup graphically, prepare, and play. No terminal, Go installation, CMD wrapper, or manual configuration editing is part of the intended player workflow.

**Start with [START-HERE.md](START-HERE.md).** This is an isolated private development candidate continuing the delivered 0.5.0 GUI checkpoint. The user reports publishing 0.4.8 to GitHub; this work does not publish or replace that release.

The 0.5.1 private candidate uses a **new isolated `private-gui-0.5.1` folder**. It does not migrate or modify the 0.5.0 test installation; choose your owned source again. Later opens of this same version reuse that installation.

## What is implemented

- A separate single-file bootstrap executable embeds a verified, offline-signed launcher package; it does not recursively package itself.
- First-run progress, visible failures, Retry/Exit and conservative interrupted-startup recovery; automatic browser opening and authenticated local session reuse on repeated clicks.
- Private per-user installation, no shortcut changes, preserved saves and generations, no changes to Steam/GOG originals.
- Graphical source/runtime selection and preparation; explicit version/core/mod choices. New GUI setup suggests Community 1.50.26 standard rules, while legacy 1.40b23 stays available and existing profiles are preserved.
- Windows GUI-subsystem builds and hidden console helpers; actual game windows are still intended to show normally.
- Official Steam/GOG acquisition handoff with honest compatibility limits; store sign-in/install remains in the official client, then return to Browse/import. No unverified full-game downloader.
- Download byte/percentage progress and responsive cancellation, with verified-cache preservation, partial-file cleanup, and retry.
- Per-profile fullscreen, starting window size, mute and background behavior controls, saved without changing game rules or requiring a rebuild.

## Boundaries

No owned game files, emulator runtime, production signing keys, telemetry, file uploads, or new network service are bundled. PRSL and a new Chat extension remain unavailable optional research, not prerequisites. No final SCG/Tournament balance or separate cruiser prototype is activated.

This source/build candidate is **not production-ready**. Linux API/process tests and cross-compilation are distinct from native Windows/macOS/Linux GUI acceptance, actual DOSBox gameplay, and multiplayer. OS signing/notarization and publication were not performed. Rendered browser acceptance is not claimed where browser startup was blocked by the execution environment.

## Developer workflow (not player instructions)

The existing Go toolchain builds `manager`, the signed helper `manager/cmd/bootstrapper`, and the outer `manager/cmd/desktop`. `packaging/build_release.py` creates the signed inner release; `packaging/build_desktop.py` accepts offline development trust only and refuses changed source fingerprints. The public-key trust is embedded at build time; private development keys are temporary and are not included in artifacts.

Relevant tests include Go race tests, `tests/ui_regression.js`, GUI-specific event regressions, and `tests/desktop_acceptance.py`. The desktop acceptance script uses disposable directories and a browser recorder, not native visual/gameplay proof. See the candidate-specific acceptance document and evidence rather than historical release claims.
