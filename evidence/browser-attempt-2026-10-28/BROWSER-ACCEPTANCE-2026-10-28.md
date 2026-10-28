# Launcher browser acceptance — 28 October 2026

**Result: blocked before UI acceptance.** The real cloud browser could not reach the launcher’s private loopback listener from its separate execution environment. No browser success is claimed.

## Tested identity

- Unreleased 0.4.8 Linux development package built 26 October; disposable offline trust with no configured download endpoints or allowed hosts.
- Launcher SHA-256: `2e0441139e3e247ff97c4bd31da13455e3b471d0c7b7bb3d8039b63722367582`.
- Package SHA-256: `fbc960840556dfdd31c2e01e507b96511aa51947fa3e81932d087fdb8b13d873`.

## Coverage

| Check | Outcome | Evidence / limit |
|---|---|---|
| Fresh isolated offline setup | Passed | Compiled setup installed the supplied signed development package successfully. |
| Launcher startup and authenticated API readiness | Passed | In the maintained launcher execution session, state returned HTTP 200 and version 0.4.8. |
| Real cloud-browser connection | Blocked | Corrected HTTP navigation after verified API readiness returned `net::ERR_CONNECTION_REFUSED`. Separate command execution also could not reach that listener. |
| Initial rendered launcher / layout | Not run | No launcher page rendered. |
| Profile navigation and settings display | Not run | Blocked by browser connection. |
| Unavailable payload/runtime error UI | Not run | Blocked by browser connection. |
| Refresh and Exit launcher UI behavior | Not run | Blocked by browser connection; terminating a tool session is not an Exit-button pass. |
| Synthetic-data preservation | Passed | Test-only save bytes matched their initial SHA-256 before cleanup. Not a loadable-save/gameplay test. |
| Original release preservation | Passed | Setup, launcher and package hashes unchanged after testing. |

No launcher UI defect was established. The actionable limitation is environment connectivity: the browser and loopback server need an authorized same-network execution context before these UI checks can run. No public relay, broader network binding, protection change, authentication workaround or Node mock was used.

## Cleanup and boundaries

The maintained tool execution session was terminated (reported exit 130), failed test tabs were closed, and the disposable install—including private URL marker and stale locks—was removed. Graceful UI shutdown was not tested. No screenshots are attached because no launcher UI rendered. This report contains no private loopback address or token.

No production install, real user saves, game/emulator, external network service, GitHub or publication was used. These results apply only to the identified October 26 build, not a later patched build.
