# Launcher 0.4.8 — current verification

**28 October 2026. Unreleased source review; not a finished installer, native-desktop certification or gameplay release.**

## What changed today

The updater previously retained the restarted launcher's private local session URL in `logs/update-handoff.log`. The fixed helper now asks the manager to keep that URL in its private recovery marker rather than standard output. Ordinary progress/error diagnostics and headless/default browser policy are retained. Initial/direct launches still print the local URL normally.

This is a local credential-retention and accidental-sharing issue, not evidence of an unauthenticated remote exploit. The API remains loopback-only with Host/Origin checks and session-token authentication.

**Important limitation:** the first update initiated by an **older installed helper** still runs that old helper and can retain its old logging behavior. The correction is proven for the fixed generation's helper and direct invocation of the new Setup's staged-apply command. Older token-bearing logs are not scrubbed, and no existing installation was upgraded or cleaned. Do not describe this as old-to-new migration acceptance.

The earlier October 26 fix also remains: `--no-browser` survives staged self-update. The package includes complete current source; it does not require applying old patches.

## Current-source results

These counts overlap. In particular, the 49-check privacy fixture includes the original 31 lifecycle checks; **do not sum the rows into a total**.

| Check | Result | What it establishes |
|---|---:|---|
| Go race suite | **243 passes; 11 private-fixture skips; 0 failures** | Tests/subtests across five tested packages; buildconfig has no tests. Not the older all-private-fixture run. |
| Offline bootstrap integration | **35 passed** | Real compiled Linux setup/manager and local API: install, verify, repair, rollback, tamper refusal, locks, same-package staged reconstruction, synthetic userdata preservation. |
| Headless handoff | **31 passed** | Opener requests: headless initial/restart **0/0**, normal **1/1**. Invocation-count-only Linux process double; no real browser. |
| Privacy-extended handoff | **49 passed** | Above lifecycle plus private-marker/ordinary-console behavior and URL/token exclusion from handoff diagnostics. Baseline fails the named regression as expected. |
| Independent extended review | **143 passed** | Direct/manual staged startup, missing/broken/echoing openers, recovery-marker failure, log-open failure, credential behavior and cleanup. Same-package Linux development scope. |
| Vet; JavaScript syntax; Node/VM UI regressions | **Passed** | Static/source and deterministic event-flow tests, not rendered-browser acceptance. |
| Publishing safeguards | **10 passed** | Mocked services only; nothing published. |
| Four launcher/setup pairs | **Compiled** | Linux amd64 executed; Windows amd64 and macOS amd64/arm64 compiled only. Disposable offline development trust. |

The tested packages contain the same **105 build-input fingerprints** as this source. Documentation and review metadata have subsequently been refreshed, so their full-tree hashes differ from the tested package's source index. `SOURCE-FINGERPRINT.json` hashes build inputs; `CANDIDATE-MANIFEST.json` identifies this documented source copy. Neither is a production signature.

Evidence: `../evidence/2026-10-28/summary.json`, `privacy49.json`, `headless31.json`, `bootstrap-integration.json`, independent runtime evidence, and test logs. The Linux development launcher used for integration has SHA-256 `cc5a6e4836437eb4aecde2bf47e3a356615a4f50e7879efd03182754c24dc4f7`; its binary is not included here.

The documented native Linux build/run recipe was additionally checked from a fresh disposable copy of this documented source: build/version, empty-root authenticated API readiness, graceful Quit, and marker/lock cleanup passed. This is a source-build/local-API smoke, not browser or gameplay acceptance. See `../evidence/2026-10-28/native-docs-smoke.json`.

## Browser attempt: blocked, not passed

A separate 28 October attempt used the **October 26** development build. Fresh setup and authenticated local API readiness passed, but the real cloud browser could not reach the private listener from its separate execution context (`ERR_CONNECTION_REFUSED`). No page rendered, so profile/settings/error/refresh/Exit UI assertions did not run. No UI defect was established. See `../evidence/browser-attempt-2026-10-28/BROWSER-ACCEPTANCE-2026-10-28.md`.

This does not certify the current source's browser UI and does not justify disabling authentication or binding the server publicly.

## Historical evidence, kept separate

- **16 October source:** 257 Go test/subtest passes with all supplied private fixtures; 50 portable/owned-file and 24 diagnostic checks using labelled runtime process doubles. These are not reruns on today's source.
- **16 October actual launcher/runtime:** final Linux source of that date launched real DOSBox Staging 0.83.0 to the 1.50.26 title screen; personal settings/fingerprint integrity held. This was title-screen startup, not a played match.
- **Separate Classic baseline:** real Classic DOSBox 0.74-3 two-client local-IPX turns/save/fresh-process reload were observed. That was not driven by today's launcher, not Staging multiplayer and not PRSL.
- **21/26 October:** stronger 35-check bootstrap acceptance and the 31-check headless correction preceded today's privacy work.

Full earlier launcher detail is in `TEST-REPORT-2026-10-16-HISTORICAL.md`. The top-level artifact index points to optional runtime/research evidence.

## Reproduction and prerequisites

Use `../START-HERE.md` for native source build and source tests. Use `DEVELOPMENT-ACCEPTANCE.md` for the Linux-only development package fixture. For the 49-check privacy version, add `--check-log-privacy` to the headless fixture command.

Go 1.23+, Python 3.11+, Node.js, writable temporary directories and local-loopback/child-process capability are needed for their respective checks. Race tests require a supported native host and C toolchain. The recorded current build used Go 1.27.3 on Linux amd64 with module network access disabled.

The following private-fixture variables are intentionally optional; relevant tests skip rather than pass if inputs are absent:

- `MOO2_COMMUNITY_FIXTURE`: prepared Community generation containing `manifest.json` and `game/`.
- `MOO2_HARNESS_FIXTURE`: the supplied Windows runtime directory for read/verify tests.
- `MOO2_TEST_ASSETS`: private test directory with the recognized supplied Steam archive and official 1.31 patch fixture. Exact historical fixture aliases are documented in the October 16 report.

The eleven skips are listed by name in the current summary. They cover real baseline/official content, private runtime, selected configuration/inventory and custom/generated-file cases. Missing private inputs do not invalidate the tests that ran, but they limit the claim. Never upload owned archives, runtime data, saves or credentials to a public repository.

## Still unestablished

Native Windows/macOS behavior; rendered browser UI; old-version migration; real-save loadability on today's build; Staging multiplayer; public relay reliability; longer matches; production signing/installation; playable PRSL or custom strategic/Tournament mods. Development package signatures validate only the disposable test trust used here. No Authenticode, notarization, production feed, publication or deployment is included.
