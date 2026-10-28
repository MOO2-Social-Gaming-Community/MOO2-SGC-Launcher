"""Generate hashes only, from the exact supported patch. No payload is embedded."""
import hashlib, json, os, zipfile
from pathlib import Path
archive = Path(os.environ['MOO2_PATCH_ZIP'])
expected = '0ac9151e9cf752ec34b6db998d8b390f48e492d8443e8f1658112b93c7f6a921'
if hashlib.sha256(archive.read_bytes()).hexdigest() != expected:
    raise SystemExit('Refusing an unrecognized patch archive')
prefix = 'MOO2-1.50.26/patch/'
with zipfile.ZipFile(archive) as z:
    records = [{'name': n[len(prefix):], 'size': len(b), 'sha256': hashlib.sha256(b).hexdigest()}
               for n in sorted(z.namelist()) if n.startswith(prefix) and not n.endswith('/')
               for b in [z.read(n)]]
(Path(__file__).parent / 'assets/patch-files.json').write_text(
    json.dumps({'schema': 1, 'edition': 'MOO2-1.50.26', 'files': records}, indent=2) + '\n')
print(len(records), 'patch file hashes written; no payload copied')
