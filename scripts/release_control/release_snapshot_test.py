"""Release provenance stays bound to the reviewed commit as its train advances."""
from copy import deepcopy
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import release_snapshot as snapshot


class SnapshotIdentityTest(unittest.TestCase):
    def setUp(self):
        self.sha = 'a' * 40
        self.merge = 'b' * 40
        self.ref = 'refs/heads/release-candidate/packet-1'
        self.pr = {
            'state': 'closed', 'merged': True, 'merge_commit_sha': self.merge,
            'head': {'sha': self.sha, 'ref': 'release-candidate/packet-1', 'repo': {'full_name': 'rcourtman/Pulse'}},
            'base': {'ref': 'release/v6.4', 'repo': {'full_name': 'rcourtman/Pulse'}},
        }

    def verify(self, pr):
        return snapshot.reviewed_merge(pr, repository='rcourtman/Pulse', ref=self.ref,
                                       branch='release/v6.4', sha=self.sha)

    def test_merged_snapshot_identity(self):
        self.assertEqual(self.merge, self.verify(self.pr))
        self.assertEqual('release/v6.4', snapshot.source_branch(self.ref, 'release/v6.4', '42'))
        self.assertEqual('main', snapshot.source_branch('refs/heads/main', '', ''))

    def test_unreviewed_or_retargeted_identity_is_rejected(self):
        changes = [('merged', False), ('state', 'open'), ('merge_commit_sha', ''),
                   ('head.sha', 'c' * 40), ('head.ref', 'release-candidate/other'),
                   ('base.ref', 'main'), ('head.repo.full_name', 'someone/Pulse'),
                   ('base.repo.full_name', 'someone/Pulse')]
        for path, value in changes:
            with self.subTest(path=path):
                pr = deepcopy(self.pr)
                target = pr
                fields = path.split('.')
                for field in fields[:-1]:
                    target = target[field]
                target[fields[-1]] = value
                with self.assertRaises(ValueError):
                    self.verify(pr)

    def test_unbound_dispatch_inputs_are_rejected(self):
        for args in [(self.ref, '', '42'), (self.ref, 'main', ''),
                     (self.ref, 'feature/other', '42'), (self.ref, 'main', '../42'),
                     ('refs/heads/main', 'main', '42'), ('refs/tags/v6.4.4-beta.1', '', '')]:
            with self.subTest(args=args), self.assertRaises(ValueError):
                snapshot.source_branch(*args)

    def test_source_workflow_supports_snapshot_inputs_before_qualification(self):
        path = Path(__file__).resolve().parents[2] / '.github/workflows/create-release.yml'
        snapshot.check_workflow(path)
        with tempfile.TemporaryDirectory() as raw:
            old = Path(raw) / 'workflow.yml'
            old.write_text(path.read_text().replace('release_pull_request:', 'removed_input:', 1))
            with self.assertRaises(ValueError):
                snapshot.check_workflow(old)

    def test_workflow_verifies_ancestry_without_requiring_current_tip(self):
        with tempfile.TemporaryDirectory() as raw:
            output = Path(raw) / 'output'
            env = {'GITHUB_REF': self.ref, 'RELEASE_SOURCE_BRANCH': 'release/v6.4',
                   'RELEASE_PULL_REQUEST': '42', 'GITHUB_REPOSITORY': 'rcourtman/Pulse',
                   'GITHUB_SHA': self.sha, 'GITHUB_WORKFLOW_SHA': self.sha,
                   'GITHUB_OUTPUT': str(output)}
            with patch.dict(os.environ, env), patch('sys.argv', ['release_snapshot.py']), \
                 patch.object(subprocess, 'check_output', return_value=json.dumps(self.pr)), \
                 patch.object(subprocess, 'run') as run:
                snapshot.main()
            self.assertEqual('source_branch=release/v6.4\n', output.read_text())
            commands = [call.args[0] for call in run.call_args_list]
            self.assertIn(['git', 'merge-base', '--is-ancestor', self.sha, self.merge], commands)
            self.assertIn(['git', 'merge-base', '--is-ancestor', self.merge, 'FETCH_HEAD'], commands)
            self.assertFalse(any('reset' in command or 'checkout' in command for command in commands))


if __name__ == '__main__':
    unittest.main()
