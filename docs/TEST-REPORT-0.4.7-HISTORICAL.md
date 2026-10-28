> Historical 0.4.7 report supplied with the original repository. Its counts and signed-build claims do not apply to the unreleased 0.4.8 candidate. See TEST-REPORT.md for current scope.

# MOO2-SGC 0.4.7 — test report

## Scope

This is an internal build/packaging cycle from the supplied 0.4.6 repository and Dopefish diagnostic kit. It does not claim that the user's Comm Failure has been resolved or that the third-party IPX service is currently reachable. No remote GitHub/account writes were made.

## Implemented and exercised

The selected profile persists in the installation's own userdata and survives application restart/new localhost ports without rewriting game data. The baseline selector no longer shows a false 1.50 Core selection. Network preview and generated game config share the same command generator. The Dopefish diagnostic runs as an independent, explicit, locked DOSBox process with no game mounts. It records request details without treating process success as network success.

The previous-version console line now clearly states it is NOT the launch target. The existing minimum-version floor, pinned-release fallback, signed validation, canonical RKERNEL preflight, source import, baseline preservation and optional Community profile remain in place.

## Results for the final signed binaries/source

| Area | Passed | Scope |
|---|---:|---|
| Go race suite | 118 top-level / 222 with subtests | Source tests; network/selection/API/integrity/distribution |
| go vet / JavaScript syntax | Passed | Static/tool checks |
| Publishing safeguards | 10 | Local unit tests, no GitHub write |
| Network diagnostic and preference API/CLI | 24 | Real Linux manager; labelled DOSBox process double |
| Browser UI | 34 | Actual manager API via explicit offline browser-transport bridge |
| Full owned-file portable/Steam/CD lineage | 50 | Actual source files and generated workspaces; no game execution |
| Old 0.4.2 kernel-missing workspace to current | 36 | Actual old/new Linux managers; driver hashes, repair and synthetic save bytes |
| Stale signed 0.4.2 feed fallback | 15 | Actual setup binaries, local HTTPS server |
| Interrupted download, signatures and restart | 15 | Local HTTPS proxy fixture, no live GitHub |
| Native signed installer/launcher | 11 | Actual Linux execution |
| Portable setup and 0.4.6 -> 0.4.7 upgrade | 20 | Actual Linux installer/launcher and existing signing identity |
| GitHub Desktop-style checkout | 10 | Local Git core.autocrlf=true, source and signature bytes preserved |
| Explicit local ZIP preparation | 9 | Actual canonical ZIP; source unchanged |

See `evidence/QA-SUMMARY.json` and the matching JSON/log files. Suites overlap; their totals are not counts of unique game scenarios.

## Explicit limitations

- Windows/macOS binaries compiled; not executed in this environment. No Windows Authenticode signature or Mac notarization supplied.
- No actual DOSBox or MOO2 engine executed. Test-double process logs are explicitly labelled. Save preservation verifies bytes, not game loadability.
- Browser direct loopback navigation was refused by the environment (`ERR_BLOCKED_BY_ADMINISTRATOR`). UI assertions used an explicit bridge to the actual HTTP API. They are not native Windows-browser acceptance.
- No live Dopefish DNS/UDP connection or multiplayer session verified. The user's screenshot of CONNECT/STATUS is the next transport evidence.
- Local HTTPS fixtures do not prove GitHub publication or live downloads. Run the publication workflow after pushing.
- First all-platform build attempt hit the tool time limit. The final build completed successfully; acceptance suites used those final signed binaries.

## Signed delivery

Manifest release **0.4.7**, revision **47**, existing `moo2-sgc-release-1` public identity; validity ends **2027-01-05T03:02:24Z**. No private key appears in the repository or portable kit. Installed verified applications retain offline operation; future downloads need valid signed metadata. The source-fingerprint check links packages to these build inputs, not to a claim of game certification.

## Next Windows acceptance

Exit old launcher/game; extract the portable kit at `C:\Games\MOO2-SGC`; run START-MOO2-SGC.cmd. Select Community — standard / 1.50.26, save Create or Join plus Dopefish. Reopen to confirm remembered profile, then run the separate connection check. Read CONNECT/STATUS, type EXIT, prepare/verify Community and launch. The baseline and owned Steam/GOG source remain separate. Keep the first error, last-network-check.json, last-launch.json and relevant screen capture.
