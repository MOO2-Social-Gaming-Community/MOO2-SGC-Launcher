> Historical release guidance. This unreleased 0.4.8 source candidate has no new signed artifacts and has not been published. Do not execute these old release/upgrade instructions for this package. See START-HERE.md and docs/TEST-REPORT.md.

# Deployment — 0.4.6

The canonical repository is `MOO2-Social-Gaming-Community/MOO2-SGC---Launcher-and-Installer`.

Primary endpoints embedded in Setup and Launcher:

```
https://github.com/MOO2-Social-Gaming-Community/MOO2-SGC---Launcher-and-Installer/releases/latest/download/manifest.json
https://github.com/MOO2-Social-Gaming-Community/MOO2-SGC---Launcher-and-Installer/releases/latest/download/manifest.sig
```

A version-pinned v0.4.6 metadata fallback is also embedded. The setup executable rejects a manifest whose selected launcher is below its local version floor, even when the signature is valid. Package URLs are version-specific. All metadata and packages require signatures from the embedded key and byte-size/hash checks. A failed mirror cannot lower an already accepted manifest revision.

## GitHub Desktop

Follow `GITHUB-UPLOAD.md`. The repo contains precompiled, signed assets solely to make the initial unzip/commit/push workflow complete. The included Action publishes them to GitHub Releases; regular clients use Releases, not raw repository files. Future maintainers may remove old prepared inputs from the default branch after publication (release assets remain archival); avoid retaining every binary version indefinitely in Git history.

The Release workflow is intentionally limited to the exact organization/repository and its default branch. It checks signatures, package contents, the standalone bootstrap against its signed copy, and source fingerprints; it uploads to a draft, reads back hashes, then publishes. A repeated workflow verifies matching existing release bytes. It can additionally repair a stale latest-release pointer; it does not demote a newer release. Different published bytes are never overwritten. A partial matching draft can be resumed.

No Cloudflare account, bucket, URL or credential was supplied. R2 remains optional and unconfigured. The advanced `packaging/publish_release.py` retains the optional mirror workflow; configure/sign actual endpoints before claiming failover is deployed. Neocities and SourceForge are not involved.

## Unavailable live verification

The development environment could not fetch the user repository or execute a hosted GitHub Actions job. Therefore default-branch name, public visibility, Actions policy and the live Release are not independently confirmed. These are deployment prerequisites, not implied completed actions. No repository writes were performed by the development session.
