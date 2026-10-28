> Historical 16 October candidate changes and evidence, superseded by the [28 October current report](TEST-REPORT.md). These counts are not tests of the current source.

# 0.4.8 candidate: unreleased changes

Source-only engineering handoff, 2026-10-16. See [current verification report](TEST-REPORT.md) for evidence, limits and reproduction.

- Pin immutable patch code/rulesets and selected configuration independently of editable local receipts.
- Include added custom configuration/data in compatibility fingerprints; conservatively reject excluded personal/audio/generated inputs reached through nested configuration references.
- Allow normal game-generated EXTRACT.CFG and ORION2.LOG without false integrity failures; preserve 150.SET and NEW150.SET personal state.
- Record runtime failures and propagate nonzero runtime completion to the CLI. Never infer network success from process exit.
- Select duplicated profiles only after successful save, preserving newer navigation.
- Add targeted integrity/reference-scanner/process/UI regression coverage and source-archive cross-build support.

Historical 16 October evidence: 257 Go test/subtest passes, 50 portable/owned-file checks, 24 diagnostic/process-double checks, 10 mocked publishing tests, deterministic UI event-flow checks, static checks and four launcher cross-builds. Counts overlap and are not unique gameplay scenarios.

Not a signed installer or multiplayer release. Browser UI acceptance was blocked by the environment and not bypassed. Windows/macOS execution, live Dopefish/multiplayer and new signed upgrades are not verified here. No game or runtime payload is distributed. 0.4.7 deployment/signing docs and artifacts are historical, not instructions to publish this candidate.

A separate real Linux launcher-to-Staging title-screen smoke identified an omitted normal 150.SET settings file. Its fingerprint/preservation classification was corrected and added to the regression coverage. See the separately scoped runtime addendum in [the historical 16 October report](TEST-REPORT-2026-10-16-HISTORICAL.md); the 50/24 automated suites remain process-double tests.
