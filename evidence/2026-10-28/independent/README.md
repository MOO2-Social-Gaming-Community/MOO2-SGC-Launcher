# Independent updater-log privacy review — 2026-10-28

**GO for the bounded fixed-generation, offline development candidate. No in-scope defects found.**

## Independent evidence

- `baseline.json` / `baseline.log`: 50 checks reproduce active restarted URL/token in the prior Oct 26 signed build's `update-handoff.log`, both headless and default modes. Prior behavior retained 0600 file permissions, 0/0 and 1/1 opener counts, stale-token rejection and process cleanup.
- `baseline-negative.{json,log,exit}`: strict fixed-behavior assertion fails with exit 1 against the same baseline at `headless handoff log private URL expectation`.
- `final-runtime.{json,log,exit}`: **143 checks pass** against the frozen patched Linux binaries. Covers signed same-package self-update, normal/headless policies, missing opener and unstartable opener, deliberately URL-echoing child stdout/stderr, private marker/log permissions, expired-token refusal, process/lock/marker cleanup, direct launcher normal/headless and explicit private mode, manual apply-staged headless guidance, marker-write failure, and update-log-open failure.
- `focused-go-security.{log,exit}`: **12 top-level tests + 2 subtests** pass with `go test -race -count=1`. Authorization, host rebinding, cross-origin rejection, no arbitrary files, allowed API shape, busy rejection, no CORS/cache leak, output policy and opener start failure are covered. The earlier narrower run is also saved as `focused-go.*`; its packages with no selected tests are not counted as test passes.
- `manifest-verify.{log,exit}`: independent `releasectl verify` validates signed revision 1 and all four packages.
- `package-source-validation.json`: all four archive hashes/sizes and all 20 signed members match; all four packaged 105-entry source fingerprints exactly match the reviewed build-input source; packaged launcher/setup bytes equal corresponding release binaries. Trust is explicitly development-only, with no endpoints or allowed hosts.
- `verified-source-lock.json`: exact reviewed source, manifest and Linux binary hashes. Source-fingerprint bytes SHA-256: `a6474d50191a8e930994448bb9eb72563ebdfa2efe1f3c4987af18c2ae281924`.
- `baseline-unchanged.json`: prior source tree's complete captured file hashes unchanged.

The implementer's full race/vet, bootstrap35/headless31/privacy49, JS and publishing-safeguard evidence was also inspected. These are separate from the independent counts above.

## Review conclusions and limits

Suppressing URL generation at the output source is preferable to redacting an updater pipe owned by the exiting parent. The private URL is still written to the private recovery marker before output guidance, and passed to the browser unless headless. Ordinary direct/startup console output remains useful. Manual `Setup --command apply-staged --no-browser` intentionally prints the exact private marker path rather than the bearer URL.

Browser start errors contain only executable/path details; opener subprocess stdout/stderr are not inherited. Independent missing-executable, invalid-interpreter and URL-echoing child tests found no updater-log token leakage. A failed recovery-file write exits without private URL output. A failed handoff-log open refuses the update, returns token-free API diagnostics, and leaves the original manager usable with its active generation and marker unchanged.

**Scope limitation:** an update initiated by an older installed launcher uses its older embedded Setup helper. That helper does not know the new output-policy flag. This candidate does not retroactively fix the first old-helper → new-launcher transition; the verified guarantee is for restarts driven by the patched helper. No heuristic broadening was added. Historical log bytes are not scrubbed, though tested old tokens are invalid after restart.

Linux process doubles and loopback HTTP only. No real browser, native Windows/macOS execution, emulator/game, PRSL, external service/network, old-release migration, publication, or production signing. Opener observation follows API readiness plus 0.7 seconds, followed by exit/descendant cleanup. Temporary captures with bearer URLs stay inside disposable 0700 test roots and are deleted, not retained as evidence. Same-user access to the recovery marker/browser process remains outside this log-privacy boundary. Local paths in diagnostics remain intentionally visible.

## Reproduce independent runtime checks

First build an isolated development package using `docs/DEVELOPMENT-ACCEPTANCE.md`. Then, from the `launcher/` source root:

```sh
TMPDIR="$PWD/review-tmp" python3 evidence/2026-10-28/independent/independent_privacy_review.py \
  --release "$PWD/review-release" --extended \
  --evidence "$PWD/review-evidence/independent-runtime.json"
```

The original October 26 release binaries are not included, so reproducing the baseline leak requires that exact separate historical development build. Historical evidence above records the baseline result without distributing binaries or credentials.
