import hashlib
import json
from pathlib import Path
import struct
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
import zipfile

ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'packaging'))
from build_review import check_gui_pe, verify_embedded
from build_desktop import trust_fingerprint
from source_fingerprint import source_index


def gui_pe():
    data=bytearray(512);data[:2]=b'MZ';struct.pack_into('<I',data,0x3c,128)
    data[128:132]=b'PE\0\0';struct.pack_into('<H',data,132,0x8664)
    struct.pack_into('<H',data,152,0x20b);struct.pack_into('<H',data,220,2)
    return bytes(data)


class ReviewPackaging(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name);(self.root/'manager').mkdir()
        (self.root/'VERSION').write_text('0.5.1\n');(self.root/'manager/example.go').write_text('package example\n')
        self.release=self.root/'release';self.release.mkdir()
        self.trust={'schema':1,'feed':'moo2-sgc-development','channel':'alpha','development':True,'keys':{'dev':'a'*64},'endpoints':[],'allowed_hosts':[]}
        (self.release/'trust-public.json').write_text(json.dumps(self.trust))
        self.manifest={'revision':1,'release':'0.5.1','expires':'2026-12-01T00:00:00Z','packages':[{'id':'launcher','kind':'launcher','platform':'windows-amd64','version':'0.5.1','filename':'package.zip','sha256':'0'*64}]}
        self.binary=self.root/'review.exe';self.extra=False
        self.make()
    def make(self):
        with zipfile.ZipFile(self.release/'package.zip','w') as z:
            for name in ('MOO2-SGC-Launcher.exe','MOO2-SGC-Setup.exe'):
                z.writestr(name,gui_pe())
            z.writestr('SOURCE-FINGERPRINT.json',json.dumps(source_index(self.root)))
            z.writestr('THIRD-PARTY-NOTICES.md','notices');z.writestr('GO-LICENSE.txt','license')
            if self.extra:z.writestr('unapproved-game.LBX','not game data; synthetic')
        (self.release/'manifest.json').write_text(json.dumps(self.manifest));(self.release/'manifest.sig').write_bytes(b'synthetic-signature-for-mocked-verifier-only')
        self.binary.write_bytes(gui_pe()+b''.join((self.release/n).read_bytes() for n in ('manifest.json','manifest.sig','package.zip')))
    def verify(self):
        with patch('build_review.subprocess.run') as verify:
            result=verify_embedded(self.release,self.binary,'windows-amd64',self.root)
            self.assertIn('verify',verify.call_args.args[0])
            return result
    def test_exact_review_payload(self):
        self.assertEqual(self.verify()['version'],'0.5.1')
    def test_changed_source_refused(self):
        (self.root/'manager/example.go').write_text('changed')
        with self.assertRaisesRegex(ValueError,'source changed'):self.verify()
    def test_unapproved_payload_members_refused(self):
        self.extra=True;self.make()
        with self.assertRaisesRegex(ValueError,'unexpected inner'):self.verify()
    def test_changed_embedded_bytes_refused(self):
        self.binary.write_bytes(self.binary.read_bytes()+ (self.release/'manifest.sig').read_bytes())
        with self.assertRaisesRegex(ValueError,'exact signed payload once'):self.verify()
    def test_wrong_platform_refused(self):
        self.manifest['packages'][0]['platform']='linux-amd64';self.make()
        with self.assertRaisesRegex(ValueError,'unexpected review package'):self.verify()
    def test_online_trust_refused(self):
        self.trust['development']=False
        (self.release/'trust-public.json').write_text(json.dumps(self.trust))
        with patch('build_review.subprocess.run') as calls:
            with self.assertRaisesRegex(ValueError,'offline development'):self.verify()
            calls.assert_not_called()
    def test_verifier_failure_propagates(self):
        with patch('build_review.subprocess.run',side_effect=subprocess.CalledProcessError(1,['verify'])):
            with self.assertRaises(subprocess.CalledProcessError):verify_embedded(self.release,self.binary,'windows-amd64',self.root)
    def test_gui_pe_required(self):
        check_gui_pe(gui_pe())
        bad=bytearray(gui_pe());struct.pack_into('<H',bad,220,3)
        with self.assertRaisesRegex(ValueError,'Windows GUI'):check_gui_pe(bytes(bad))
        with self.assertRaises(ValueError):check_gui_pe(b'truncated')
    def test_trust_identity_map_order_stable(self):
        self.trust['keys']={'z':'b'*64,'a':'c'*64};a=trust_fingerprint(self.trust)
        self.trust['keys']={'a':'c'*64,'z':'b'*64};self.assertEqual(a,trust_fingerprint(self.trust))
        self.trust['keys']['a']='d'*64;self.assertNotEqual(a,trust_fingerprint(self.trust))
    def test_existing_output_not_overwritten(self):
        sentinel=self.root/'keep';sentinel.write_bytes(b'preserve')
        result=subprocess.run([sys.executable,str(ROOT/'packaging/build_review.py'),'--out',str(self.root),'--revision','1'],capture_output=True,text=True)
        self.assertNotEqual(result.returncode,0);self.assertIn('Refusing existing output',result.stderr)
        self.assertEqual(sentinel.read_bytes(),b'preserve')
    def test_workflow_never_publishes(self):
        workflow=(ROOT/'.github/workflows/release.yml').read_text()
        self.assertIn('contents: read',workflow);self.assertIn('upload-artifact@',workflow)
        self.assertIn('--run-native-smoke',workflow);self.assertNotIn('contents: write',workflow)
        for bad in ('--execute','gh release','GH_TOKEN','secrets.','publish_prebuilt.py'):
            self.assertNotIn(bad,workflow)
        tests=(ROOT/'.github/workflows/test.yml').read_text()
        for suite in ('ui_regression','gui_onboarding_regression','presentation_regression','stability_regression'):
            self.assertIn(suite+'.js',tests)
        self.assertNotIn('native_smoke.py',tests)
    def test_generated_and_owned_artifacts_ignored(self):
        repo=self.root/'ignore-check';repo.mkdir()
        subprocess.run(['git','init','--quiet',str(repo)],check=True)
        (repo/'.gitignore').write_bytes((ROOT/'.gitignore').read_bytes())
        paths=['release/0.5.1/app.exe','release/0.5.1/private.pem','release-assets/windows-review/MOO2-SGC-Windows.exe','game/ORION2.EXE','base.zip']
        result=subprocess.run(['git','check-ignore','--stdin'],input='\n'.join(paths)+'\n',cwd=repo,text=True,capture_output=True,check=True)
        self.assertEqual(result.stdout.splitlines(),paths)

if __name__=='__main__':unittest.main()
