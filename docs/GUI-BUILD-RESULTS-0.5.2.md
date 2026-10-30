# 0.5.2 Windows stability candidate — verified build results

Based on the delivered 0.5.1 source checkpoint. All older delivered EXEs, source
ZIPs and Library identities remain unchanged. No GitHub publication was performed.

## Final reviewed identity

- Source inputs: 150 files; `SOURCE-FINGERPRINT.json` SHA256
  `8e7602eafd62f8a6c67eef9443d1aaba7f51a1e13b6d07cf8bd34e5565692033`.
- Windows EXE: 15,236,096 bytes; SHA256
  `e37a4a6d17d41f98daf974c27d38de13127052b15fe30e62b85ba5d106069315`.
- Delivered filename: `MOO2-SGC-0.5.2-Windows.exe`. The clean CI bundle uses
  `MOO2-SGC-Windows.exe`; the delivered copy has identical bytes.
- Go 1.27.3; signed offline development revision 2026103001.
- Embedded manifest expires 2027-01-28 02:19:07 UTC. This is not an enduring
  stable public installer or a provisioned production updater/signing identity.

## Passed on the exact final application source

- Independent full fixture-enabled Go race suite: **367 tests/subtests passed,
  zero failed, zero test skips**, across seven tested packages. The buildconfig
  package has no tests. Private inputs included verified Steam, manual baseline,
  original 1.31 patch, Community patch and runtime fixtures; none are distributed.
- Go vet passed. All seven test-bearing packages cross-compiled for Windows;
  those Windows test binaries were not executed here.
- Four Node/VM suites and two JavaScript syntax checks passed. These are logic
  tests, not browser rendering.
- 22 Python packaging/publishing safeguards passed. Publication tests are mocks.
- Actual final Linux one-file process/API smoke: **11 checks**, no browser opened.
- Actual final Linux lifecycle: **30 checks**, browser opener is only a recorder.
- Two independently signed builds of the same version/revision: **8 checks**,
  proving separate default roots, explicit-root refusal, same-file reuse and
  unchanged synthetic save/ownership/generation/session data.
- Independent Ed25519 verification of final Windows/Linux manifests and packages,
  package member hashes, exact five-member inner inventory, current source index
  and exact signed bytes embedded once in each outer executable.
- Windows outer, manager and helper are x64 PE GUI subsystem 2; no Authenticode.

The optional build-review evidence-retention flag was the only indexed change
after the final Go run; Python checks and actual full builds were rerun for it.
No Go/application source changed after the final aggregate.

## Not run / not approved

Native Windows execution, a real GitHub Actions run for this source, rendered
browser/picker acceptance, actual DOSBox visibility/audio/gameplay/save/load,
multiplayer and native macOS are not certified. A new configured Windows CI job
is not evidence it has already run. Independent review approves a **private
candidate only**, not a stable public download.

The manager-crash/possibly surviving game case still preserves protective locks
when exit history is unknown. The known failure before game start is now
journaled safely; it does not solve that broader crash-recovery design. Stable
signing, update/migration and expiry policy are also unresolved. See
[STABLE-RELEASE-GATE.md](STABLE-RELEASE-GATE.md).

## Source push versus executable

The exact delivered 0.5.1 source ignored compiled EXEs and lacked a matching
prepared release folder; its old workflow did not build the GUI outer. That
explains why a source-only push was not enough to produce this app. The known
public repository URL was checked read-only, but the accessible snapshot was
three weeks old and current Actions/Releases/API responses were unavailable.
The user's later push was therefore not independently verified or contradicted.

The new workflow is read-only for repository contents, builds the actual outer
and uploads a **temporary Actions artifact**, not a Release. The maintainer
website steps and manual Draft Release asset path are in GITHUB-UPLOAD.md.
