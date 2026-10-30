# Windows stable-release gate

This is hardening work based on the delivered 0.5.1 checkpoint, not a declaration
that it is stable. Native process/API checks are not visible Windows acceptance.
Nothing here authorizes publication or changes the delivered 0.5.1 bytes.

## Automated review pipeline

- Test Go code with race/vet on Linux; test Go natively on the Windows runner.
- Run all four JavaScript logic suites (not a rendered browser).
- Build the signed inner package, then embed it in the separate outer GUI EXE.
- Verify signatures, exact embedded bytes, current source fingerprint, a five-file
  inner package allowlist, and GUI subsystem for outer/manager/Setup Windows PEs.
- Run the actual outer and installed manager on a Windows runner with explicit
  `--no-browser`: isolated Unicode path, first install, auth fence, second open,
  missing game/runtime, unchanged synthetic save, restart and clean exit.
- Produce a clean four-file review bundle; upload it only as an Actions artifact.

The native runner steps are implemented but are **not verified as run** until a
real workflow result for the exact commit is recorded. Current cloud Linux
results cannot certify Windows. Private fixture tests cannot run in public CI
without separate lawful inputs; skipped tests must remain visible.

## Required native Windows 11 review of exact EXE hash

Record Windows/browser versions and EXE SHA256. Use a fresh private installation
and owned source; leave the working launcher/Steam/GOG sources and saves alone.

1. Double-click: no terminal, visible startup progress, automatic graphical
   chooser; second click reuses the same installation/window session.
2. Browse folder/ZIP: cancel cleanly, choose the verified owned 1.40 source;
   unsupported/missing source must show an actionable error, not Ready.
3. Prepare Community 1.50.26 standard (legacy is an explicit separate choice).
   Check progress, network loss, Cancel and retry, checksum failure, runtime
   import/download. Failed/stale results must not enable Play.
4. Save fullscreen/window-size/audio/background choices; restart and verify
   persistence. Launch a visible game window, confirm audio/focus behavior,
   start/save/load a disposable game, then quit cleanly.
5. Reopen and verify the same setup/save. Test browser close/reopen, repeated
   clicks and a canceled file picker. Capture visible error text if anything
   fails. No commands should be necessary for ordinary player operation.
6. Verify original owned-source files and existing working installation remain
   unchanged. Exercise antivirus/SmartScreen warnings without disabling them;
   unsigned-build warning absence is not promised.

Actual DOSBox/gameplay/audio/save and rendered GUI remain open until observed.
No production-ready, multiplayer, macOS or Ubuntu one-open claim follows from
cross-compilation or process doubles.

## Explicit blockers before a stable public link

- A production signing/update identity and lifecycle, including OS signing
  decision, durable installer availability and signed-manifest expiry handling.
  Current offline development builds expire and are intentionally isolated by
  version + public trust fingerprint. Rebuilding does not migrate user data.
- Safe crash recovery while a game may survive its manager. Existing protective
  locks intentionally remain when process history is unknown. Do not delete
  them based only on a dead manager PID: DOSBox may still be running. Recovery
  needs independently validated process creation identity and durable start/exit
  history (including the pre-PID crash window), or another reviewed design.
- A supported, data-preserving update/migration path from real released versions;
  private-root isolation is not that path.
- Exact native Windows visual/game/save acceptance and independent final review.
- Verified authorization/checksums before adding a full-game download source.
  Until then use owned-source import and official store handoff only.

## If interrupted startup refuses recovery

Close any visible game/launcher windows, then use the startup screen's recovery
button only if it allows it. If it still reports unknown history, stop using
that test installation and retain it for diagnosis; **do not delete locks or
saves, run an old installer over it, or disable security protection**. Report
its exact error and whether Windows was restarted. The current conservative
refusal is a known limitation, not a promise that Retry or reboot repairs it.

## Maintainer-only local build (not player instructions)

With the documented Go/Python toolchain available:

```text
python packaging/build_review.py --revision 1 --out release-assets/review-1
```

On an actual Windows x64 host add `--run-native-smoke`. The builder refuses an
existing output directory and production trust. It can cross-build Windows on
Linux, but that is explicitly not native testing. Other targets remain internal
architecture checks. Reproducibility here means repeatable source-to-bundle
steps and verified provenance, not identical bytes across freshly generated
keys/timestamps. Retain the exact reviewed bundle, not a later rebuild.
