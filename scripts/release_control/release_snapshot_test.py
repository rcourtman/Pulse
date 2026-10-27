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

    def test_post_merge_ref_identifies_its_merged_snapshot_by_head_name(self):
        dispatch = 'refs/heads/release-dispatch/packet-1'
        self.assertEqual('release/v6.4', snapshot.source_branch(dispatch, 'release/v6.4', '42'))
        self.assertEqual(self.merge, snapshot.reviewed_merge(
            self.pr, repository='rcourtman/Pulse', ref=dispatch, branch='release/v6.4', sha=self.sha))
        for head_ref in ('release-candidate/other', 'release-dispatch/packet-1', 'maintainer/packet-1'):
            with self.subTest(head_ref=head_ref), self.assertRaises(ValueError):
                pr = deepcopy(self.pr)
                pr['head']['ref'] = head_ref
                snapshot.reviewed_merge(pr, repository='rcourtman/Pulse', ref=dispatch,
                                        branch='release/v6.4', sha=self.sha)

    def test_snapshot_head_admits_only_reserved_snapshot_names(self):
        self.assertEqual('release-candidate/v6.6.1-rc.1-20261101T000000Z',
                         snapshot.snapshot_head('release-dispatch/v6.6.1-rc.1-20261101T000000Z'))
        self.assertEqual('release-candidate/packet-1', snapshot.snapshot_head('release-candidate/packet-1'))
        for branch in ('release-dispatch/', 'release-dispatch/a/b', 'release-dispatch/a b',
                       'maintainer/packet-1', 'main', 'xrelease-dispatch/packet-1'):
            with self.subTest(branch=branch), self.assertRaises(ValueError):
                snapshot.snapshot_head(branch)

    def test_patch_snapshot_requires_its_reviewed_version_bound_target(self):
        ref = 'refs/heads/release-dispatch/v6.6.1-rc.1-stamp'
        branch = 'release/v6.6.1'
        self.assertEqual(branch, snapshot.source_branch(ref, branch, '42'))
        with self.assertRaisesRegex(ValueError, 'reviewed immutable snapshot'):
            snapshot.source_branch('refs/heads/' + branch, '', '')
        # Preserve direct dispatch on pre-train historical source lines.
        self.assertEqual('release/v6.3.2', snapshot.source_branch('refs/heads/release/v6.3.2', '', ''))
        pr = deepcopy(self.pr)
        pr['head']['ref'] = 'release-candidate/v6.6.1-rc.1-stamp'
        pr['base']['ref'] = branch
        self.assertEqual(self.merge, snapshot.reviewed_merge(
            pr, repository='rcourtman/Pulse', ref=ref, branch=branch, sha=self.sha))
        with self.assertRaises(ValueError):
            snapshot.reviewed_merge(pr, repository='rcourtman/Pulse', ref=ref,
                                    branch='release/v6.6.2', sha=self.sha)

    def test_unbound_dispatch_inputs_are_rejected(self):
        dispatch = 'refs/heads/release-dispatch/packet-1'
        for args in [(self.ref, '', '42'), (self.ref, 'main', ''),
                     (self.ref, 'feature/other', '42'), (self.ref, 'main', '../42'),
                     (dispatch, '', '42'), (dispatch, 'main', ''), (dispatch, 'feature/other', '42'),
                     ('refs/heads/release-dispatch/a/b', 'main', '42'),
                     ('refs/tags/release-dispatch/packet-1', 'main', '42'),
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
        for ref in (self.ref, 'refs/heads/release-dispatch/packet-1'):
            with self.subTest(ref=ref):
                self.verify_main(ref)

    def verify_main(self, ref):
        with tempfile.TemporaryDirectory() as raw:
            output = Path(raw) / 'output'
            env = {'GITHUB_REF': ref, 'RELEASE_SOURCE_BRANCH': 'release/v6.4',
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
