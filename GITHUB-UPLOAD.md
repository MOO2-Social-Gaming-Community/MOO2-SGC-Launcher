> Historical release guidance. This unreleased 0.4.8 source candidate has no new signed artifacts and has not been published. Do not execute these old release/upgrade instructions for this package. See START-HERE.md and docs/TEST-REPORT.md.

# GitHub Desktop upload — 0.4.7

Extract the complete repository ZIP contents into the cloned launcher repository, preserve `.git`, commit and push the default branch. `release/0.4.7` contains ready-signed release files. Wait for **Publish prepared release** and confirm a published **v0.4.7**.

Do not rename/repack signed packages or edit manifest.json. Same signing identity as 0.4.1/0.4.2; no new signing secret required to publish the supplied artifacts. The publishing workflow checks source fingerprints, signatures and exact uploaded bytes. It does not certify MOO2 gameplay.

The portable integration ZIP is for C:\Games\MOO2-SGC, NOT for the repository. Keep the game ZIP/runtime/saves/private key outside your clone. Git ignore patterns defend against accidental inclusion, but review GitHub Desktop changes before committing.

Older release folders already present in your clone can remain; VERSION selects 0.4.7 for publication. The standalone setup retrieves the published release. The separate offline integration kit can be tested before publication and retains online update support afterward.


0.4.7 verifies the latest-release pointer after publication and repairs it when it still refers to an older release. It never deliberately moves latest backward. The installer also checks its own version floor and retries its pinned endpoint, so stale latest no longer selects 0.4.2. The offline kit is the recommended recovery test when publication is uncertain.
