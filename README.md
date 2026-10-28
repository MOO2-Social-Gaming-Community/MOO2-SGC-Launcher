# MOO2-SGC launcher — 0.4.8 source review

**28 October 2026. Complete, unreleased source. Not an installer or gameplay certification.**

Start with [START-HERE](START-HERE.md) for native build/run instructions, then the [current test report](docs/TEST-REPORT.md). This review combines the earlier integrity/settings/runtime fixes, stronger offline bootstrap acceptance, headless update handoff, and the newly reviewed update-log privacy correction.

## Current improvements

- Authenticate prepared patch/base content against compiled identities and account for custom configuration dependencies.
- Preserve ordinary personal settings during repair and report process start/exit failures accurately.
- Keep duplicate-profile selection consistent during asynchronous actions.
- Preserve `--no-browser` across staged update handoff.
- With the fixed updater helper, omit the restarted launcher's private session URL from `update-handoff.log` while retaining ordinary diagnostics and the private local recovery marker.

**Important transition limit:** the first update executed by an older helper can still use its old logging behavior. This source does not sanitize historical logs or certify an old-version migration. See the test report before making upgrade claims.

The independent 1.40b23 baseline and optional 1.50.26 Community profile remain separate. PRSL, new Chat and SGC Online remain unavailable. There are no new gameplay presets, no game/runtime/save payloads, no private keys, and no release binaries here.

Use the current source fingerprint for build inputs and the candidate manifest for this documented copy. Older reports are explicitly dated; the October 16 real-emulator and Classic multiplayer evidence is not a new test of this October 28 source. Windows/macOS are compiled only, and browser acceptance remains unestablished.
