# Start here: MOO2-SGC 0.5.2 private GUI candidate

**The goal is an app, not a terminal workflow.** This development candidate continues the delivered 0.5.1 launcher; it does not overwrite that checkpoint or older releases.

## For a player on Windows 11 (x64)

Use the matching **single-file `MOO2-SGC-0.5.2-Windows.exe`** supplied with the private candidate. Double-click it. Do not install Go, run CMD wrappers, or edit configuration files.

1. A browser window shows verification and setup progress. The launcher installs only itself into a separate per-user private folder. It does not replace an existing launcher or shortcut.
2. If needed, follow the official Steam/GOG store links to obtain your owned game. Sign-in, purchase and installation stay with the official store/client; this candidate does not download a complete game from a third-party backup site. Then return to the launcher. In the graphical chooser, **Browse** to a supported owned game folder or ZIP. Source files are verified and copied; Steam/GOG originals and their saves are not modified or uploaded. Not every regional or GOG edition has a verified recognition recipe.
3. Choose the game version and community ruleset you want. For a new GUI user, **Community 1.50.26 / standard rules** is suggested; **legacy 1.40b23** remains an independent option. Existing saved selections are retained. No experimental SCG/Tournament balance is selected automatically.
4. Use **Prepare**. Missing supported community patch/runtime files can be obtained through the visible verified-download flow, or choose an existing supported runtime through the graphical controls. Launcher installation itself is offline; game preparation can need those additional files or internet access.
5. Review the readiness result. Optional **Display, sound and background settings** provide fullscreen, starting window size, mute and single-player background pause. Save settings or choose **Play**; these presentation changes do not require preparing the game again. Preparation never starts a game automatically.

Downloads show received bytes/percentage when total size is supplied. **Cancel preparation** interrupts downloads; verification and short activation steps stop only at safe boundaries. Failed or canceled downloads keep the previous verified cache and prepared game.

**Separate test setup:** 0.5.2 uses the per-user `MOO2-SGC/private-gui-0.5.2-<public-trust-fingerprint>` folder, distinct from 0.5.0/0.5.1 and independently rebuilt development keys. Choose the owned source again; no old test profiles or saves are migrated automatically. Keep the old EXE if you need to return to that old test installation.

Later, double-click the same file. It reuses the private installation; a second click reopens the current session rather than starting another manager. Use **Exit launcher** when finished. Closing a browser tab alone does not stop a running game or launcher.

If setup stops, its page provides **Retry**, **Recover interrupted startup**, and **Exit**. Recovery never clears a live/unknown process lock. If a previous game has no confirmed exit record, recovery deliberately stops rather than risking its files. A missing default browser or unwritable private folder is shown as an error; no terminal command is part of normal recovery.

## Important limits

- This is a **private development candidate**, not a production-ready release. Windows/macOS builds are cross-compiled, not native-tested here; Windows Authenticode and macOS notarization are absent. Do not bypass OS security warnings or enterprise policy.
- No game data, emulator, PRSL adapter, or new Chat extension is embedded. The original in-game chat is unchanged. The separate newSCG cruiser prototype is not activated.
- Windows is the first native acceptance target. macOS and Linux build architecture is retained, but native desktop launch, pickers, runtime behavior, and packaging still need their own acceptance checks.
- Do not copy this source tree or its test outputs over a working installation or saves. No assistant GitHub publication/deployment was performed.

## For maintainers only

This source tree is not itself the single-file executable. `packaging/build_release.py --development` produces disposable offline-signed inner packages; `packaging/build_desktop.py` verifies and embeds a matching package in a separate outer executable. The builder refuses a package whose source fingerprint no longer matches the tree. Those scripts require the maintainer's existing Go toolchain; **players do not**.

Read `docs/STABLE-RELEASE-GATE.md` and the final build/test evidence before distributing a native test candidate. Historical 0.4.x documents describe their own artifacts and are not 0.5.2 acceptance evidence.

For the source-push/EXE distinction and Actions download steps, read [GITHUB-UPLOAD.md](GITHUB-UPLOAD.md). Nothing is automatically published.
