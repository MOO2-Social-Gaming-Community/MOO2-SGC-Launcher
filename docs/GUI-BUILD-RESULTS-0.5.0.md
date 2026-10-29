# 0.5.0 private GUI candidate — final build evidence

Date: 2026-10-28 UTC. Scope: isolated development candidate, **not a production release or native Windows/gameplay certification**.

## Result

The single-file entry point is implemented and built for Windows amd64, Linux amd64, macOS amd64, and macOS arm64. Windows is the first intended native acceptance target. The executable contains a separately signed offline launcher package, never a recursive copy of itself. No commercial game data, runtime, saved game, or private signing key is included.

| Check | Final result |
|---|---|
| Independent Go race suite | **331 compiled-source test/subtest passes plus 3 reviewer-only startup-failure assertions (334 checks total), 0 failed, 3 explicit fixture gaps**, across 7 tested packages. The gaps require genuine official-1.31-dependent inputs; they are not counted as passes. |
| Independent package/security audit | **59 checks passed**: Ed25519 signatures, hashes, signed member inventories, embedded payload identity, source fingerprint, and PE subsystem. |
| Compiled Linux one-file lifecycle | **30 checks passed**, independently repeated in fresh private roots. Browser opening was a recorder, not a rendered browser. No game or emulator was launched. |
| Go vet; JavaScript syntax; both Node/VM UI regression suites | Passed on final source. |
| Four platform builds | Passed; Linux built on Linux. Windows and macOS were cross-compiled, not executed natively. |
| Windows executable subsystem | Outer app, installed launcher, and signed Setup helper all have PE subsystem **2 (GUI)**. Runtime console suppression deliberately does not request `SW_HIDE` for the game window. |
| Production publication / OS signing | Not performed. Development payload signatures are not Windows Authenticode or macOS notarization. |

The compiled lifecycle covers no-argument first open, automatic local interface handoff, isolated roots, repeat-click reuse, preserved synthetic saves, normal GUI and explicit-headless update policies, clean exit, later reopen, outer-process crash/reopen, read-only and unowned-folder refusal, concurrent first clicks, live/unknown-runtime lock preservation, conservative recovery, and tampered embedded-payload refusal. Private session credentials are not retained in logs.

## Provenance

- Baseline source: reviewed **0.4.8**, fingerprint `c2441988ae9b3fc3feb7771946c9d9fb1fc8e276953df62360c75739b2dc7e95`. The user reports publishing 0.4.8 separately; this work did not modify that release.
- Final indexed source: **137 inputs**, fingerprint `467874d729a66f904234ae02ca16f63297b1014ff8b531f76f62a59e64af3caf` (SHA-256 of the pretty-printed `SOURCE-FINGERPRINT.json` bytes).
- Windows single-file candidate: `MOO2-SGC-windows-amd64.exe`, **15,228,416 bytes**, SHA-256 `6cd937f3963f8aed892e9cf78ef0f6fbd0fd6c6c286e6dd554f387a678402e40`.
- The public signing key is embedded; the disposable development private key was generated outside release outputs and deleted after signing. Offline-only trust has no production update endpoints.

Reproducible build metadata and concise lifecycle/PE results are in `evidence/gui-0.5.0/`. Historical 0.4.x evidence retained elsewhere describes its own earlier source, not this candidate.

## Still required before a public app release

Native Windows 11 testing must confirm file dialogs, browser/second-click behavior, absence of console flashes, actual game-window visibility, both source/version paths, save preservation, cancellation/recovery, and real gameplay. macOS/Linux desktop packaging and native dialogs need separate acceptance. Rendered browser testing was blocked before UI assertions in this execution environment. OS signing, reputation, broad GOG-edition support, and multiplayer were not established. See `GUI-FIRST-ACCEPTANCE-0.5.0.md` for the native checklist.

The tested candidate neither enables PRSL/new Chat nor selects an experimental SCG/Tournament balance or cruiser prototype.
