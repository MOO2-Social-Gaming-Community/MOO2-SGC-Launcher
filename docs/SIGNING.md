> Historical release guidance. This unreleased 0.4.8 source candidate has no new signed artifacts and has not been published. Do not execute these old release/upgrade instructions for this package. See START-HERE.md and docs/TEST-REPORT.md.

# Release signing — 0.4.7

Existing `moo2-sgc-release-1` public identity, feed `moo2-sgc`, channel `stable`, revision 47. The previously stored private backup remains valid. No replacement key is distributed; never commit private signing material.

Prepared metadata expires **2027-01-05T03:02:24Z**. Verified installations may run offline; installs/updates after expiry require renewed signed metadata. Do not edit signatures or turn off freshness checks.

The old 0.4.6 installer accepted this new release in the native upgrade tests. Package Ed25519 signatures are distinct from Windows Authenticode/macOS notarization, which are not supplied. The private build key was kept outside the repository and removed from this cycle's working directory after signing. Publication uploads already-signed assets and requires no new GitHub signing secret for this release.
