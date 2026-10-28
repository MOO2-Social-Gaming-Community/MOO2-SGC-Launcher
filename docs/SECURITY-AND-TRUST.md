> Historical release guidance. This unreleased 0.4.8 source candidate has no new signed artifacts and has not been published. Do not execute these old release/upgrade instructions for this package. See START-HERE.md and docs/TEST-REPORT.md.

# Security and trust — 0.4.1

- Release metadata and each executable package require an embedded-key Ed25519 signature, declared byte count and SHA-256 identity.
- Release metadata also signs download locations. Redirects stay within approved HTTPS hosts.
- Staging, installed-file verification, startup/version checks, operation locks and retained previous versions are separate steps.
- First public download authenticity still depends on a trusted delivery channel; files are not Windows Authenticode signed or macOS notarized. Signature verification inside Setup does not independently authenticate a replaced Setup binary.
- No commercial game archive, signing private key or service credential belongs in the public repository.
- Local UI uses a per-session bearer token, loopback binding, strict Host/Origin checks and a restrictive content-security policy. Its private URL is excluded from diagnostic reports.
- A chosen existing DOSBox executable is user-trusted, not magically certified by discovery. Official runtime downloads use pinned upstream hashes. Runtime installation does not establish that gameplay works.
- Folder import copies only the recognized fingerprint list and rejects mismatches, symlinks and case collisions. It does not execute imported installers or upload files.
- Archives are extracted into operation-owned staging areas; traversal, duplicate names and size-limit violations are refused.
- Signed metadata lasts 90 days in this release. Expired metadata cannot authorize a new install, though an existing verified installation can run offline.
- A version label without an alpha/beta suffix is not a security or gameplay certification. Build compiler and test limitations are recorded in the release/test report.

See `SIGNING.md` for private-key custody and deliberate key rotation. Do not bypass the checks to make a failing test pass.
