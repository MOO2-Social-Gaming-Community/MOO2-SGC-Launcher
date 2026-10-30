# Focused stability audit — 2026-10-30

Scope: harden the delivered 0.5.1 Windows-first launcher before a permanent download link. This working increment is separate from the immutable delivered 0.5.1 source/artifacts. No native Windows acceptance, stable-release certification, public upload, engine change or new game-download host is implied.

## Fixed, with failure reproduced before the change

1. **Identical quick retry result was skipped.** The GUI identified completion using operation kind/content only. If a repeated action finished before a busy-state poll and returned the same error/result, it could be ignored after the action cleared the prior error. A repeat folder selection could similarly be skipped. Jobs now have monotonically increasing, per-manager-process numeric IDs, and the GUI includes this ID in completion identity. The number is not a credential/user identifier and is not persisted. A repeat poll of the *same* completed job still does not replay it.
2. **A failed poll could be overtaken by an old successful resolve.** Disconnection now blocks mutating controls and readiness, increments the preview revision and clears cached preview identity. Existing asynchronous responses cannot restore readiness. Reconnection requires a successful fresh state response **and** fresh selected-profile resolution before Play. An action already waiting on profile-selection persistence also rechecks connection state before sending.

3. **Known pre-start failure left ambiguous runtime history.** Launch wrote its journal, then a failed presentation-settings save returned before attempting DOSBox without recording that fact. This is now explicitly marked as a known no-start failure. A journal-write failure still aborts launch and reports both errors; it does not allow recovery or claim a process exit. Retrying after fixing the preference destination creates a fresh record.

Changed production files: `manager/manager.go` (job identity only), `manager/web/app.js` (completion/reconnection guards), `manager/runtime.go` and `manager/process_result.go` (known pre-start failure journaling). No authentication, hash-verification, runtime lock or process-lifetime safety rule was relaxed.

## Tests

- `tests/stability_regression.js`: actual application functions with deterministic DOM/network doubles. Covers identical retry errors, repeated identical picker success, no same-job replay, stale resolve after failed poll, fresh recovery, old response arriving after recovery, and failure while an action waits for selection persistence.
- `manager/job_identity_test.go`: unique IDs for identical completed failures, no identity change on busy rejection/completion, no identity persistence across manager restart.
- `manager/prestart_history_test.go`: a copied, verified private Community fixture plus an explicitly labelled Unix process double reproduces the preference-failure/no-start journal bug, proves no process ran, then proves retry works. A separate fixture-free test verifies journal-write failure remains an error with no partial file. This is not native Windows/gameplay acceptance.
- Existing onboarding, presentation and duplicate Node suites remain required, now **four** Node suites in total. Relevant Go race tests are required before freeze; independent aggregate/Windows CI results are separate evidence.

This is control-flow testing, not browser rendering or execution of Windows DOSBox.

## Remaining prioritized release gates

1. **Native Windows 11:** clean first open without terminal/Go/admin, double-open/reuse, native picker focus/cancel/Unicode, prepare/cancel/retry with pinned real assets, visible DOSBox window, settings persistence and actual sound/fullscreen/background behavior, game exit/reopen, DPI and keyboard navigation. Cross-compilation/process doubles do not close these gates.
2. **Crash while runtime is running:** `cmd/desktop/recovery.go` correctly refuses to clear locks without a confirmed runtime exit/start-failure record. A dead manager can leave DOSBox alive. The current history does not retain a verifiable child process/start identity or boot identity, so a manager crash during gameplay can leave later opens unable to recover automatically even after the game has ended or the machine restarted. Do not fix this by deleting/ignoring locks or assuming a dead manager means a dead game. A separate trustworthy runtime-lifetime/reboot recovery design and Windows tests are required.
3. **Settings partial commit:** presentation preferences can be written before a subsequent selected-profile write fails. The error is visible and retryable; prior preferences are not guaranteed unchanged after such an error. No game files/saves are corrupted. This known lower-priority consistency limitation was deliberately not expanded into a new multi-file transaction here.
4. **Coverage boundaries:** genuine 1.31/legacy private-fixture suites remain required rather than silently skipped. Their results, actual gameplay/LAN/public-IPX, OS signing/SmartScreen behavior and rendered browser acceptance must be reported separately; passing source/unit/lifecycle tests does not certify the latter.

The artifact publication/build pipeline is audited separately. Source pushes do not by themselves prove a built EXE was uploaded to a release or validated on Windows.

Final independent 0.5.2 aggregate: 367 Go test/subtest passes, zero test skips, using verified private fixtures including genuine legacy lineage. This closes the former cloud fixture gaps only, not the native/gameplay gates above.
