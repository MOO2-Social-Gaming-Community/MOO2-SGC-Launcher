# GUI-first launcher: 0.5.0 private candidate

The next app-oriented version is **0.5.0**, following the user's reported 0.4.8 GitHub release. This work does not modify that release or authorize publication. The supplied 0.4.8 source tree remains separate. Source compilation is a maintainer task, not an end-user setup step.

## Smallest implemented graphical path

1. Open the single platform launcher file. Its isolated desktop bootstrap prepares its embedded signed application and opens the authenticated local interface. It does not install over `C:\Games\MOO2-SGC`.
2. Choose **Browse game folder**, **Browse game ZIP**, or **Find installed MOO2**. File selection is native on Windows; no path typing, archive renaming, moving files, shell command, or configuration editing is required. Canceling the chooser preserves the previous selection.
3. A fresh desktop session recommends **Community — standard**, engine **1.50.26**, core **150**, no extra mods, standalone. A remembered choice wins. The independent **1.40b23 baseline** and historical versions remain explicit options. No new custom balance, PRSL, Chat, or online service is silently enabled.
4. **Prepare selected game** performs the complete existing pipeline: copy and verify recognized owned DOS files; check the allowed version lineage; obtain required pinned patch/runtime packages; prepare and verify a separate generation. Both visible Prepare controls use this same pipeline. The original game folder/ZIP is not patched or uploaded.
5. The ready card identifies the selected profile/version/ruleset. **Play MOO2** is unavailable for an unprepared profile, changed version/mods, unresolved conflict, pending new source selection, missing runtime, or invalid required kernel. Actual game/runtime hashes are checked again at launch. Preparation never starts gameplay automatically.
6. Close MOO2 normally, then choose **Exit launcher**. Closing only the browser tab leaves the local process running; opening the same outer launcher file is the intended return route.

## What changed beyond packaging

- Authenticated file-chooser actions use fixed dialog types, never user-supplied shell commands. Windows uses native Win32 dialogs and does not invoke PowerShell. Other desktop platforms have separate, less-tested chooser adapters.
- Source discovery remains a bounded scan of known Steam/GOG roots and Steam library locations. Zero or multiple candidates are normal; Browse is the fallback, not a request to rearrange source files.
- **GOG is not universally certified.** Search may find GOG folders, but acceptance still requires the compiled recognized English DOS file inventories. Unsupported assets fail without modifying originals. A GOG-specific edition has not been added or invented.
- Ruleset and add-on options remain derived from the embedded 1.50.26 catalog. Changing rules invalidates readiness until the chosen profile is prepared. Loaded saves can restore their own gameplay configuration; use a new game to test changed rules.
- Runtime detection no longer means verified readiness. Portable Windows ignores arbitrary global executables and uses the managed pinned bundle. The status explicitly distinguishes metadata inspection from full pre-launch checksum verification.
- Ordinary users may obtain the **checksum-pinned official DOSBox Staging 0.83.0 archive** through Prepare or Download / repair. The archived official package was compared directly with the embedded inventory: its pinned archive hash and all **660 runtime files** match. A previously supplied private harness is **not a prerequisite**.
- Optional runtime folder/ZIP import copies only verified files into this isolated installation. It never moves or modifies the selected input. Repair stages the replacement and retains the prior managed tree; corrupt/partial runtime folders no longer require manual deletion.
- Cancel requests are honored at safe boundaries after the current download or verification step. Activation transactions reject late cancellation. Existing prepared game generations remain retained; completed source-cache or runtime preparation can safely be reused on retry.
- UI polling is single-flight. Resolution results are tied to the selected profile and preview revision; an old response cannot re-enable Play after a newer change. Repeated clicks cannot submit a second simultaneous operation.
- Desktop startup/update failures have graphical reporting. Normal desktop startup uses one-time `--defer-browser-once` so the parent owns only the initial redirect; an update then opens its own authenticated page. Explicit engineering `--no-browser` remains independent and is preserved across updates, even together with `--desktop-gui`. Actual DOSBox processes use CREATE_NO_WINDOW only, without requesting SW_HIDE for their game window; setup/manager helpers remain hidden. These structural flags do not constitute a native game-window test.
- A cheap authenticated `/api/identity` route supports outer-launcher session discovery without polling every game/runtime file.

## Evidence and limits

Implementation-stage checks performed on the shared Linux executor:

| Check | Result and scope |
|---|---|
| Go aggregate `go test -race -count=1 ./...` | Passed, with real supplied-harness and original-official-runtime ZIP fixtures enabled. Original game-fixture-dependent tests may skip when their separate inputs are absent. The final packaged source has a separate release-build verification gate. |
| `go vet ./...` | Passed at the implementation checkpoint. |
| Windows amd64 test cross-compilation | Passed, including native-picker source. This is not Windows execution. |
| JavaScript syntax | Passed. |
| Existing duplicate-profile Node/VM test | Passed. |
| New GUI-onboarding Node/VM test | Passed: ready-state gates, pending source, chooser cancellation/Unicode path transfer, stale responses/conflicts, repeated clicks, single-flight polling, both Prepare routes, and required DOM IDs. |
| Real runtime import/repair fixtures | Passed: supplied folder, supplied harness ZIP, original official ZIP, exact hashes, retained damaged-tree backup, unchanged input bytes. No emulator execution or public download was performed by these checks. |
| Native Windows dialogs/browser lifecycle/gameplay | **Not executed.** Native Windows is unavailable in this executor. |
| Direct browser visual acceptance | **Not passed.** Earlier browser attempts were blocked before UI assertions; Node/VM checks do not replace browser acceptance. |
| Real Steam/GOG/CD-to-gameplay Windows journey | **Not certified.** GOG-specific contents remain unvalidated. |
| Public release, signing identity approval, SmartScreen reputation | **Not established by these source checks.** Private development signatures are not Windows Authenticode. |

The fixture-enabled review identified three optional 1.31-dependent coverage gaps. In particular, the legacy-receipt reconstruction case needs genuine `ORION131.EXE` bytes, which a verified Steam 1.40b23 → Community generation legitimately does not contain. That case skips only for a fully verified, recognized Steam lineage with that file absent; missing or wrong bytes in an explicit legacy fixture remain failures. Set `MOO2_LEGACY_COMMUNITY_FIXTURE` to a genuine legacy-derived prepared Community generation to run it. No 1.31 executable was synthesized and no production identity check was weakened. Final aggregate counts and these skips must be reported separately.

Private runtime inputs and Library materialization helpers are outside `work/` and must never be included in source or public launcher packages. A final source snapshot, embedded signature/member verification, single-file integration run, and corresponding versioned evidence are owned by the packaging stage; do not reuse an earlier staging signature after source edits.

## Required Windows 11 acceptance checklist

Use a separate installation and copies of expendable test saves, never an irreplaceable working setup.

### First open and source choice

- [ ] A fresh Windows 11 account with no Go, Python, terminal setup or manual config opens the single file and reaches the GUI; no console window appears.
- [ ] Default-browser absence/failure produces an actionable graphical error, not a disappearing console.
- [ ] No changes appear in the existing 0.4.8 installation, Steam/GOG installation, original archives, or unrelated shortcuts.
- [ ] Browse dialogs appear in front, support keyboard navigation, paths with spaces/non-ASCII characters, read-only sources, Cancel, and reopening after Cancel.
- [ ] Steam default and alternate-library candidates can be found; zero/multiple candidates lead naturally to Browse/explicit selection.
- [ ] A recognized owned ZIP works after renaming; wrong folders, incomplete files and unsupported GOG/retail variants fail clearly without modifying them.

### Prepare, runtime and profiles

- [ ] New onboarding selects Community standard 1.50.26/core150/no add-ons; explicitly choosing baseline stays baseline. Relaunch preserves the user's saved choice.
- [ ] A supported source plus no runtime can prepare using the pinned official runtime download without possessing the old private harness.
- [ ] Offline, failed, truncated or wrong-hash download reports a recoverable GUI failure; retry or GUI package/runtime import succeeds without shell commands.
- [ ] Importing an existing verified runtime folder/ZIP leaves input hashes unchanged. Partial/corrupt managed runtime repair retains the old tree and restores launchability.
- [ ] Progress remains visible; repeat clicks do not duplicate work. Cancel before commit stops safely, late commit cannot be canceled, and retry succeeds.
- [ ] Changed engine/rules/add-ons or a newly selected source disables Play until prepared; stale responses cannot restore old readiness. Conflicting catalog options have useful errors.
- [ ] Preparation interruption/repair retains the prior active generation and same-engine saves/settings. Retained-generation recovery works from the GUI.

### Play, exit and return

- [ ] Play starts the selected prepared game; observe **Version 1.50.26** in the real game for the Community-standard case, not just a successful process start.
- [ ] Complete a new-game turn, save, quit, relaunch and load that save. Verify actual keyboard/mouse, rendering, audio and fullscreen behavior.
- [ ] Nonzero process exit and startup failure are visibly distinct from normal exit and never labeled gameplay success.
- [ ] Close the browser tab, double-click the same outer launcher, and return to the same authorized session without a second manager or manual URL/lock editing.
- [ ] Repeat open while preparing and while the game runs; no competing manager or duplicate game starts.
- [ ] Explicit Exit stops the manager and releases normal session locks. The next open restores the selected profile without downloading/rebuilding unnecessarily.
- [ ] Crash recovery is conservative when game-process status is uncertain; no live runtime/game locks are deleted merely because the browser or manager disappeared.
- [ ] Optional diagnostics and updates remain secondary; no account, public IPX connection, PRSL or Chat is required for ordinary single-player setup.

Cross-platform packaging, actual multiplayer, PRSL/Chat and final balance design are separate gates; none is implied by this GUI onboarding work.
