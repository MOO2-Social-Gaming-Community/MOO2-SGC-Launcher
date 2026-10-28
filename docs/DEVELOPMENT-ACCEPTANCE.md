# Development acceptance — advanced, Linux only

This is an optional maintainer workflow, not the first review step. It runs actual Linux setup/launcher processes with temporary synthetic userdata and local HTTP. It does **not** execute a real browser, game, emulator or external relay. The Windows/macOS outputs are compiled only.

## Prerequisites

- Linux amd64; Go 1.23+, Python 3.11+; a C toolchain for `go test -race`.
- Writable, disposable directories; permission to create loopback listeners and child processes.
- No game or runtime fixtures are required for these two integration tests.
- Do not reuse a working MOO2 installation, an existing release directory, or production signing credentials.

From the `launcher/` source root in a new extracted copy:

```sh
mkdir -p review-tmp review-evidence review-gocache
export TMPDIR="$PWD/review-tmp"
export GOCACHE="$PWD/review-gocache"
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
# review-release must not exist or must be empty.
python3 packaging/build_release.py --development --revision 1 --out "$PWD/review-release"
python3 tests/headless_handoff.py --release "$PWD/review-release" --check-log-privacy --evidence "$PWD/review-evidence/headless-privacy.json"
python3 tests/bootstrap_integration.py --release "$PWD/review-release" --evidence "$PWD/review-evidence/bootstrap"
```

The builder creates a disposable offline-only signing identity, embeds its public trust, and deletes the temporary private key on exit. It refuses a nonempty output directory. Revision 1 is the bootstrap fixture's convention. Do not publish or relabel its output as production.

`headless_handoff.py` installs an `xdg-open` **call-count-only process double** before starting setup. A headless initial/restarted manager must request 0/0 opener calls; default mode must request 1/1. This is not real-browser acceptance. The fixture owns cleanup of its child processes and checks locks/private markers disappear.

`bootstrap_integration.py` tests signed install/verify, repair, rollback, tamper refusal, lock handling and same-package staged reconstruction. Synthetic save/profile/settings sentinels test byte preservation only. They are not real MOO2 saves. These tests do not establish an upgrade from an older version.

Do not share captured stdout containing a launcher URL. Use the authored sanitized result JSONs and reviewed logs. If a test is interrupted, confirm all of its owned processes stopped before removing temporary directories or locks. Stop on a policy denial; do not bypass it.

For broader private-fixture tests and historical scope, read `TEST-REPORT.md`. For current code changes and exact evidence identities, use the source fingerprint and candidate manifest in the source root.
