# Why the EXE is not in a source push

This repository deliberately ignores compiled EXEs and ZIPs. That protects the
source tree from generated files, owned game files, caches and signing keys.
GitHub's automatically generated **Source code (zip)** is source, not an app.
Do not remove those ignore rules or commit game/runtime data to fix the download.

The configured repository is
`MOO2-Social-Gaming-Community/MOO2-SGC---Launcher-and-Installer` (project.json).
This document does not claim that a particular commit or release is published.

## New review build — GitHub website, no local developer tools

1. Upload/commit the complete source tree, including `.github/workflows`, to your
   repository. Keep your clone's `.git` folder. Do not copy private fixtures.
2. Open **Actions → Windows review build (no publication)**. A push or pull
   request starts it; **Run workflow** can also start a fresh review build.
3. Wait for both source verification and Windows review build jobs to pass.
   A failed job produces no approved download; open its failed step for details.
4. In that run's **Artifacts** section, download `MOO2-SGC-Windows-review-…`.
   Extract it. The player file inside is **MOO2-SGC-Windows.exe**. The other
   files record hashes, build identity and test limitations.

**Actions artifacts are not Releases and are not a permanent public download
link.** They expire after 14 days under this workflow. Download and retain the
exact reviewed bundle. A new run creates new offline development trust and a
separate private installation; it does not migrate saves from earlier builds.

The workflow has read-only repository permission. It does not create a release,
change tags/latest, use publishing secrets, or publish a stable download. The
historical publishing scripts are not called by either current workflow.

## Draft release assets — only after review

Before a release, choose a new numeric version for changed player code. Never
replace a delivered version's EXE with different bytes. Follow
[the release gate](docs/STABLE-RELEASE-GATE.md), including native visual/gameplay
acceptance and the production trust/update decision.

Then, using **Releases → Draft a new release**, target the exact reviewed commit
and attach the exact `MOO2-SGC-Windows.exe`, `SHA256SUMS.txt`, `BUILD-RESULTS.json`
and `READ-ME-FIRST.txt` from the same successful build. Save as **Draft**. The
EXE must be an attached binary asset: GitHub does not compile the source ZIP for
you. Verify its hash after downloading the draft asset. Do not click Publish or
mark Latest until the release gate and explicit publication approval are met.

This private-review builder is **not a production updater/signing solution**.
Its embedded offline manifest expires, and each build has disposable trust.
Do not advertise its EXE as an enduring stable installer or use it to replace
an older production setup. No production key was provisioned by this work.

Players only download/open the approved Windows EXE. The source archive,
Python/Go commands and internal Setup helper are for maintainers, not players.

Official references: [GitHub release assets and drafts](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository),
[Actions artifacts and retention](https://docs.github.com/en/actions/tutorials/store-and-share-data).
