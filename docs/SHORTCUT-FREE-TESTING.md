# Shortcut-free CLI testing — unreleased October 28 development source

`--no-shortcut` tells this version of Setup not to create or replace a platform shortcut. Setup passes the flag to the launcher; the launcher passes it to each staged updater; that updater passes it to the restarted launcher. It stays enabled through repeated updates started from that running session. The flag is also accepted by direct launcher invocation.

Use it **with an explicit, separate `--install-root`**. Neither this flag alone nor an isolated root alone is a complete isolated-test entry point: the flag does not change the installation root, and normal Windows Setup can replace the shared `MOO2-SGC.lnk` shortcut even with a separate root. The private test-kit wrappers must always supply both. Do not run the generic test-kit builder's default-root wrappers for an isolated test.

Example from an unpacked private Windows kit containing the new Setup and matching signed offline `release` folder:

```bat
MOO2-SGC-Setup.exe --install-root ".\test-install" --offline ".\release" --no-shortcut
MOO2-SGC-Setup.exe --install-root ".\test-install" --command launch-installed --no-shortcut
```

The same flag must be included for manual `repair`, `rollback`, `ensure-installed`, and `apply-staged` commands that launch the UI. Private wrappers should include it on every invocation, including `verify` and `--no-launch`, so later wrapper edits cannot accidentally re-enable shortcut writing. Use absolute kit-local paths in those wrappers; the relative paths above are CLI examples only.

For direct launcher execution, pass `--no-shortcut` alongside the installation's explicit `--app-root`, `--root`, and `--data` paths. A direct launcher does not itself write shortcuts, but its later update helper normally can.

## Deliberately unchanged behavior

- This is per-process policy, **not saved configuration**. After quitting, reopening without the flag restores normal behavior. Double-clicking Setup directly does not opt in; use the kit wrapper. No old shortcut is removed or rewritten in shortcut-free mode.
- Without the flag, Setup retains its normal shortcut behavior and normal installation-root selection. No automatic portable-layout policy was added.
- `--no-browser` remains independent: include it only when no browser should be opened. Shortcut-free mode may still open the browser normally.
- Staged-update restarts retain the private-URL recovery file rather than logging the session URL. Ordinary initial/direct launches retain their existing console output.
- This requires the updated Setup, launcher, and signed update helper. No shortcut-free transition from old installed binaries is certified; do not reuse older signed packages as if built from this source.

Only offline development signing and disposable installations are used for validation. Linux process tests and cross-compilation do not certify Windows desktop behavior, SmartScreen acceptance, gameplay, Steam/GOG integration, or production release readiness. Existing Steam, GOG, and installed MOO2-SGC folders must not be selected as the test root.
