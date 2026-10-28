# Launcher 0.4.8 — current source review, 28 October 2026

**Complete source; unreleased. Not a drop-in update or installer.** Keep your working installation, original archives and saves unchanged. The current status is in `docs/TEST-REPORT.md`; dated reports under `docs/history/` retain their historical scope.

## Build a native review binary

Prerequisite: Go **1.23 or newer** available as `go` (`go version` should identify the compiler). The module uses the Go standard library, so source compilation does not need game files or module downloads. Extract this package into a **new, separate folder**. Run these commands from `launcher/`.

### Linux or macOS shell

```sh
mkdir -p review-bin review-root review-data
(cd manager && go build -buildvcs=false -trimpath -o ../review-bin/moo2-manager .)
./review-bin/moo2-manager --version
./review-bin/moo2-manager --root "$PWD/review-root" --data "$PWD/review-data" --no-browser
```

### Windows PowerShell

```powershell
New-Item -ItemType Directory -Force review-bin, review-root, review-data | Out-Null
Push-Location manager
go build -buildvcs=false -trimpath -o ../review-bin/moo2-manager.exe .
Pop-Location
.\review-bin\moo2-manager.exe --version
.\review-bin\moo2-manager.exe --root "$PWD\review-root" --data "$PWD\review-data" --no-browser
```

Stop if the build fails; do not continue with a binary left over from an earlier attempt. Windows/macOS instructions are the source's native build route, **not a claim that they were executed here**.

The manager prints a private loopback URL and writes `review-data/open-launcher.txt` while running. To preview the UI, manually open that URL in a browser on the same computer. Do not share the URL or the marker: its fragment is a session credential. `--no-browser` suppresses automatic opening, not the local server. Use the UI Quit action or Ctrl+C when finished; closing the browser tab alone does not stop the manager.

Without owned game archives and an appropriate runtime, you can inspect the launcher but cannot play. This build has no provisioned online release feed. Do not try to apply an existing signed 0.4.7 package to this changed source or regard an old signature as authenticating it.

## Automated source checks

Python 3.11+ and Node.js are needed for their respective checks. Go's race detector also needs a supported native host and C toolchain. From `launcher/`:

```sh
(cd manager && go test -race -count=1 ./... && go vet ./...)
node --check manager/web/app.js
node tests/ui_regression.js
python3 -m unittest discover -s tests -p 'test_publish*.py' -v
```

These checks use authored fixtures and loopback services; some Go tests explicitly **skip** without private supplied game/runtime fixtures. See `docs/TEST-REPORT.md` for exact counts and prerequisites. Do not set private-fixture paths to your only copy of a working install.

For advanced Linux-only offline bootstrap/handoff reproduction, see `docs/DEVELOPMENT-ACCEPTANCE.md`. It builds a temporary development-signed test release, not a production installer. `sh manager/build.sh` is the optional four-target cross-build route; cross-build success does not prove native execution.

## Review order

Source/static checks → isolated launcher/UI inspection → owned-copy real runtime/save tests → native-platform and migration acceptance → separate multiplayer checks. A signed release is a later maintainer decision. No GitHub push, publication, production signing, live PRSL hook or gameplay rule activation is authorized by this review package.
