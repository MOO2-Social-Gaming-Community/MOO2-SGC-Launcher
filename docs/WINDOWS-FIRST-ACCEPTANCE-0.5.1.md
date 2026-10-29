# Windows-first acquisition and settings — private candidate 0.5.1

This increment preserves the delivered 0.5.0 checkpoint. The default 0.5.1 private application root is `MOO2-SGC/private-gui-0.5.1`, separate from 0.5.0. This is a fresh setup: select your owned game again. No automatic profile/save migration or signing-trust migration is claimed. Do not point the old 0.5.0 executable at profiles newly written by 0.5.1.

## What the GUI now does

- Provides fixed official Steam and GOG store/help links, followed by **I've installed it — find my game**, which runs actual local discovery. Browse folder/ZIP remains available. Original files are read-only inputs, never rearranged or uploaded.
- Explains that the base game is commercial. Store sign-in, purchase and entitled game installation remain in the official store/client. The launcher neither collects store credentials nor grants ownership. This is a graphical store handoff, **not an integrated commercial-game downloader**.
- Distinguishes automatic approved free patch/runtime downloads from commercial game acquisition. There is no unverified third-party full-game backup link.
- Identifies the supported English DOS source and warns that GOG-specific editions remain unvalidated. GOG EXE/BIN installers are not parsed or executed. Matching known contents are required; recognition is not based on a store name alone.
- Adds per-profile fullscreen, starting window size, mute audio, background mute and background pause. Fixed choices are mapped to DOSBox configuration, not raw user-provided config text. Pause on focus loss applies only to single-player, so switching to the launcher does not pause a network session.
- Provides **Save settings**, **Cancel edits** and **Restore defaults**. Defaults change the form only until saved or played. Play also persists these presentation preferences before starting DOSBox. Saving preferences does not save unrelated unsaved engine/mod/network changes or alter game files/saves. Settings and last selected profile survive restart.
- Retains old profiles' exact default runtime configuration and rules fingerprints. Changing presentation does not require a game rebuild.
- Handles blocked browser session storage without crashing: the authenticated fragment remains available for reload when storage cannot retain it. External links use `noopener noreferrer`; the local server still enforces token, host and origin checks.

## Automated evidence in source

`manager/presentation_settings_test.go` covers old-profile defaults, actual runtime configuration/arguments, network pause suppression, rules fingerprint invariance, all supported size choices, rejected/injected values, scoped save/restart/selection, unrelated profile preservation, deterministic save failure/retry and HTTP authentication/schema checks.

`tests/presentation_regression.js` uses deterministic JavaScript DOM/event doubles for defaults/Cancel, unsaved profile navigation, failed/stale saves, newer edits, profile reopen, store-to-discovery wiring, safe fixed links and blocked-storage authentication. Existing duplicate/readiness/onboarding suites must also pass. These are **not rendered-browser acceptance**.

## Required native Windows 11 acceptance (still open)

1. Open the single candidate EXE from a normal folder, without Go, a terminal, admin privileges or Python. Initial progress and authenticated manager appear; a second open reuses the same private installation.
2. Follow Steam's official handoff, complete installation in Steam, return and discover/import. Select a supported owned portable ZIP/folder instead and confirm no original file changes. Verify wrong/unrecognized editions give an actionable error without replacing a prepared copy. Treat GOG as an explicit unvalidated-edition test, not a promised passing route.
3. Prepare recommended Community 1.50.26; inspect actual patch/runtime download progress. Cancel a slow download; verify prompt safe interruption and retained previous generation/cache. Retry and verify success before Play becomes available. Cancellation during verification/activation respects transaction boundaries.
4. Choose each display/audio setting; Save; close/reopen the launcher and verify persisted values/selected profile. Observe fullscreen/window sizing and sound in the actual game window. Windows DPI scaling can affect logical window sizes; native rendering/audio behavior is not established by unit tests.
5. Change settings, then Cancel; restore defaults then Cancel; switch profiles and decline/accept discard. Verify each profile retains its own values. Simulate save failure and ensure visible error with retry, no silent success.
6. Play with unsaved presentation changes, exit the game, reopen the launcher and confirm saved preferences. Ensure the game window is visible and no console appears. Test launch failure with a preference-write failure: no game process may start.
7. Switch to host/join with background pause selected: the preference is retained but the session must continue when unfocused. Do not infer actual LAN/public-IPX success from a started process.
8. Block browser session storage and reopen the authenticated page; setup should work without a script crash. Test an unauthenticated page for visible reopen guidance. Check keyboard focus, native picker focus/cancel/Unicode paths, 125–200% DPI, small screens and tab order.
9. Exit the launcher; reopen; verify no stale process/root conflicts. Test explicit headless policy separately from normal GUI handoff/update. Keep the isolated 0.5.0 installation unchanged.

Ubuntu and macOS runtime/rendering/install acceptance remain later work. PRSL, new Chat, SGC Online, replacement engines and automatic commercial-game account integration are not implemented by this increment.

## Primary references checked 2026-10-29

- [Official Steam MOO2 page](https://store.steampowered.com/app/410980/Master_of_Orion_2/) and [Steam installation help](https://help.steampowered.com/en/faqs/view/6F74-68BA-1B77-8727).
- [Official GOG MOO1+2 page](https://www.gog.com/en/game/master_of_orion_1_2) and [GOG download help](https://support.gog.com/hc/en-us/articles/213148105-How-do-I-download-my-purchased-items). The help page body did not render in the research tool; the GUI does not depend on guessed account-menu instructions.
- DOSBox Staging [display/window](https://www.dosbox-staging.org/0.83/manual/graphics/display-and-window/), [mixer](https://www.dosbox-staging.org/0.83/manual/sound/mixer/) and [system](https://www.dosbox-staging.org/0.83/manual/system/overview/) configuration references. These support the fixed configuration mapping; they do not substitute for native testing of the pinned runtime.
