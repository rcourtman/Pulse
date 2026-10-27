"""Whole merge results and exact controls, with real Git but no network or publishing."""
from copy import deepcopy
from datetime import datetime, timezone
from fnmatch import fnmatchcase
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

import stable_patch as patch
from control_plane import release_branch_for_version


class StablePatchTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / 'pulse'
        self.root.mkdir()
        self.git('init', '--quiet', '-b', 'main')
        self.write('app', 'stable\n')
        self.write('other', 'stable\n')
        self.write('.github/workflows/old.yml', 'old')
        self.write('scripts/release_control/control', 'old')
        self.commit('stable')
        self.base = self.head()
        self.git('tag', 'v6.6.0')
        self.write('app', 'fixed\n')
        self.commit('fix')
        self.fix = self.head()
        self.write('other', 'unrelated main feature\n')
        self.write('.github/workflows/old.yml', 'reviewed main control')
        self.write('scripts/release_control/control', 'reviewed main control')
        self.commit('main work and controls')
        self.control = self.head()
        self.git('update-ref', 'refs/remotes/origin/main', self.control)
        self.selection = {'schema_version': 1, 'version': '6.6.1', 'stable_tag': 'v6.6.0',
                          'stable_sha': self.base, 'control_sha': self.control,
                          'fixes': [{'number': 12, 'merge_sha': self.fix, 'issues': [1]}]}

    def git(self, *args):
        return patch.value(self.root, *args)

    def head(self):
        return self.git('rev-parse', 'HEAD')

    def write(self, path, text):
        target = self.root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text)

    def commit(self, message):
        self.git('add', '--all')
        patch.commit(self.root, message)

    def assemble(self):
        return patch.assemble(self.root, self.selection)

    def test_reconstructs_all_selected_source_but_no_other_main_work(self):
        receipt = self.assemble()
        self.assertEqual('fixed\n', (self.root / 'app').read_text())
        self.assertEqual('stable\n', (self.root / 'other').read_text())
        self.assertEqual('reviewed main control', (self.root / '.github/workflows/old.yml').read_text())
        self.assertEqual(self.selection['fixes'], receipt['applied'])
        self.assertEqual([], receipt['skipped'])
        self.write('VERSION', '6.6.1-rc.1\n')
        self.commit('release metadata')
        self.assertEqual(receipt, patch.validate(self.root, '6.6.1-rc.1', 'release/v6.6.1'))
        # This must not accidentally be an ancestor of main or change main's pins.
        self.assertEqual(self.control, self.git('rev-parse', 'main'))

    def test_rejects_extra_source_even_when_manifest_claims_the_right_fix(self):
        self.assemble()
        self.write('other', 'injected feature')
        self.commit('extra source')
        with self.assertRaisesRegex(ValueError, 'unselected source: other'):
            patch.validate(self.root, '6.6.1', 'release/v6.6.1')

    def test_rejects_foreign_controls_wrong_branch_and_second_rc(self):
        self.assemble()
        with self.assertRaisesRegex(ValueError, 'version-bound'):
            patch.validate(self.root, '6.6.1', 'main')
        with self.assertRaises(ValueError):
            patch.validate(self.root, '6.6.1-rc.2', 'release/v6.6.1')
        self.write('scripts/release_control/control', 'foreign control')
        self.commit('control drift')
        with self.assertRaisesRegex(ValueError, 'unselected source'):
            patch.validate(self.root, '6.6.1', 'release/v6.6.1')

    def test_whole_merge_result_not_just_the_last_individual_commit(self):
        self.git('checkout', '--quiet', '-b', 'feature', self.control)
        self.write('new-a', 'one')
        self.commit('feature first')
        self.write('new-b', 'two')
        self.commit('feature second')
        self.git('checkout', '--quiet', 'main')
        self.git('merge', '--quiet', '--no-ff', 'feature', '-m', 'merged fix')
        merge = self.head()
        self.selection['control_sha'] = merge
        self.selection['fixes'] = [{'number': 13, 'merge_sha': merge, 'issues': [1]}]
        result = self.assemble()
        self.assertEqual(self.selection['fixes'], result['applied'])
        self.assertEqual('one', (self.root / 'new-a').read_text())
        self.assertEqual('two', (self.root / 'new-b').read_text())
        self.assertEqual('stable\n', (self.root / 'app').read_text())

    def test_real_conflict_is_skipped_without_losing_later_clean_fixes(self):
        self.write('app', 'third state after main fix\n')
        self.commit('second change depends on first')
        conflict = self.head()
        self.write('new', 'clean fix\n')
        self.commit('independent clean change')
        clean = self.head()
        self.selection['control_sha'] = clean
        self.selection['fixes'] = [{'number': 13, 'merge_sha': conflict, 'issues': [1]},
                                   {'number': 14, 'merge_sha': clean, 'issues': [2]}]
        receipt = self.assemble()
        self.assertEqual([14], [f['number'] for f in receipt['applied']])
        self.assertEqual([13], [f['number'] for f in receipt['skipped']])
        self.assertEqual(['app'], receipt['skipped'][0]['conflicts'])
        self.assertEqual('stable\n', (self.root / 'app').read_text())
        self.assertEqual('clean fix\n', (self.root / 'new').read_text())
        self.git('update-ref', 'refs/remotes/origin/main', clean)
        self.assertEqual(receipt, patch.validate(self.root, '6.6.1-rc.1', 'release/v6.6.1'))

    def test_next_patch_can_start_from_non_main_patch_source(self):
        self.assemble()
        self.git('tag', 'v6.6.1')
        prior = self.head()
        self.git('checkout', '--quiet', 'main')
        self.write('later', 'next fix')
        self.commit('next main fix')
        merge = self.head()
        self.selection.update(version='6.6.2', stable_tag='v6.6.1', stable_sha=prior,
                              control_sha=merge, fixes=[{'number': 15, 'merge_sha': merge, 'issues': [2]}])
        receipt = self.assemble()
        self.git('update-ref', 'refs/remotes/origin/main', merge)
        self.assertEqual(receipt, patch.validate(self.root, '6.6.2', 'release/v6.6.2'))
        self.assertEqual('fixed\n', (self.root / 'app').read_text())
        self.assertEqual('next fix', (self.root / 'later').read_text())

    def test_incomplete_invalid_duplicate_out_of_order_selection_is_not_assembly(self):
        for update in ({'version': '6.6.2'}, {'stable_sha': self.fix}, {'fixes': []},
                       {'fixes': self.selection['fixes'] * 2}, {'schema_version': 99}):
            with self.subTest(update=update), self.assertRaises((ValueError, subprocess.CalledProcessError)):
                patch.validate_selection(self.root, {**self.selection, **update})

    def test_source_guard_runs_before_payload_creation_and_in_canonical_ci(self):
        repo = Path(__file__).resolve().parents[2]
        workflow = (repo / '.github/workflows/create-release.yml').read_text()
        self.assertLess(workflow.index('name: Reconstruct stable patch source'), workflow.index('name: Validate release-note visual plan'))
        self.assertIn('python3 scripts/release_control/stable_patch.py validate', workflow)
        self.assertIn('python3 scripts/release_control/stable_patch_test.py',
                      (repo / '.github/workflows/canonical-governance.yml').read_text())

    def test_assembly_does_not_disable_installed_git_hooks(self):
        with mock.patch.object(patch.subprocess, 'run') as run:
            patch.git(self.root, 'commit', '-m', 'fixture')
        argv = run.call_args.args[0]
        self.assertFalse(any('hooksPath' in value for value in argv))

    def test_dirty_checkout_is_not_discarded(self):
        self.write('untracked', 'do not discard')
        with self.assertRaisesRegex(ValueError, 'clean disposable'):
            self.assemble()
        self.assertEqual('do not discard', (self.root / 'untracked').read_text())

    def activation_evidence(self, *, tag='v6.6.0', age=86400):
        now = 1800000000
        row = {'tag_name': tag, 'draft': False, 'prerelease': False,
               'published_at': datetime.fromtimestamp(now - age, timezone.utc).isoformat()}
        return now, row

    def test_activation_rechecks_exact_base_and_late_clock_for_rc_and_stable(self):
        self.assemble()
        stamp, stable = self.activation_evidence()
        for version in ('v6.6.1-rc.1', 'v6.6.1'):
            for elapsed in (0, 3 * 86400 - 1, 3 * 86400, 9 * 86400):
                def read(path):
                    # The clock can advance while API collection is in flight.
                    clock.return_value = stamp + elapsed
                    if path == 'releases?per_page=100&page=1':
                        return [stable]
                    self.assertEqual('commits/v6.6.0', path)
                    return {'sha': self.base}
                with self.subTest(version=version, elapsed=elapsed), \
                        mock.patch.object(patch, 'api', side_effect=read), \
                        mock.patch.object(patch.time, 'time', return_value=stamp) as clock:
                    if elapsed >= 3 * 86400 and version.endswith('-rc.1'):
                        with self.assertRaisesRegex(ValueError, '48 hours'):
                            patch.activation_check(self.root, version, self.head())
                    else:
                        result = patch.activation_check(self.root, version, self.head())
                        self.assertEqual((True, self.base, stamp + elapsed),
                                         (result['applicable'], result['stable_sha'], result['observed_at']))
                        self.assertEqual(version.endswith('-rc.1'), result['new_cut'])

    def test_new_minor_and_base_tag_drift_stop_activation_after_source_admission(self):
        self.assemble()
        # A valid reconstruction is not a durable publication permission.
        patch.validate(self.root, '6.6.1-rc.1', 'release/v6.6.1')
        stamp, stable = self.activation_evidence()
        for rows, sha, message in (([stable, {**stable, 'tag_name': 'v6.7.0'}], self.base, 'newer stable'),
                                   ([stable], self.fix, 'source changed')):
            with self.subTest(message=message), mock.patch.object(patch, 'api', side_effect=[rows, {'sha': sha}]), \
                    mock.patch.object(patch.time, 'time', return_value=stamp), self.assertRaisesRegex(ValueError, message):
                patch.activation_check(self.root, '6.6.1', self.head())

    def test_activation_has_no_minor_beta_or_historical_patch_gate(self):
        with mock.patch.object(patch, 'api', side_effect=AssertionError('no patch API read')):
            for version in ('6.7.0', 'v6.7.0-rc.1', 'v6.7.0-beta.1', 'v6.4.5', 'v6.4.5-rc.6'):
                self.assertEqual({'applicable': False}, patch.activation_check(self.root, version, ''))

    def test_activation_cannot_use_missing_or_other_source_manifest_or_second_rc(self):
        self.assemble()
        with mock.patch.object(patch, 'api', side_effect=AssertionError('local binding first')):
            for version, sha in (('6.6.1', self.base), ('6.6.2', self.head()),
                                 ('6.6.1-rc.2', self.head()), ('6.6.1-beta.1', self.head()), ('6.6.1', 'main')):
                with self.subTest(version=version, sha=sha), self.assertRaises((ValueError, subprocess.CalledProcessError)):
                    patch.activation_check(self.root, version, sha)

    def test_inventory_and_anchor_failures_never_turn_into_permission(self):
        self.assemble()
        stamp, stable = self.activation_evidence()
        malformed = [None, {}, [None], [{'tag_name': 'v6.7.0'}], [],
                     [{**stable, 'published_at': '2026-10-27T10:00:00'}]]
        for rows in malformed:
            with self.subTest(rows=rows), mock.patch.object(patch, 'api', return_value=rows), \
                    mock.patch.object(patch.time, 'time', return_value=stamp), self.assertRaises((ValueError, TypeError)):
                patch.activation_check(self.root, '6.6.1', self.head())
        with mock.patch.object(patch, 'api', side_effect=subprocess.CalledProcessError(1, 'gh')) as api, \
                self.assertRaises(subprocess.CalledProcessError):
            patch.activation_check(self.root, '6.6.1', self.head())
        api.assert_called_once()  # no fallback/retry after a failed evidence read

    def test_pagination_cannot_hide_newer_stable_behind_previews(self):
        self.assemble()
        _, stable = self.activation_evidence()
        preview = {**stable, 'prerelease': True, 'tag_name': 'v6.7.0-beta.1'}
        with mock.patch.object(patch, 'api', side_effect=[[preview] * 100,
                                                       [stable, {**stable, 'tag_name': 'v6.7.0'}]]) as api, \
                self.assertRaisesRegex(ValueError, 'newer stable'):
            patch.activation_check(self.root, '6.6.1', self.head())
        self.assertEqual('releases?per_page=100&page=2', api.call_args.args[0])

    def test_later_patch_uses_minor_publication_not_recent_patch_clock(self):
        self.assemble()
        self.git('tag', 'v6.6.1')
        prior = self.head()
        self.git('checkout', '--quiet', 'main')
        self.write('later', 'next fix')
        self.commit('next main fix')
        merge = self.head()
        self.selection.update(version='6.6.2', stable_tag='v6.6.1', stable_sha=prior,
                              control_sha=merge, fixes=[{'number': 15, 'merge_sha': merge, 'issues': [2]}])
        self.assemble()
        stamp, stable = self.activation_evidence(tag='v6.6.1', age=60)
        _, anchor = self.activation_evidence(age=4 * 86400)
        for observed_anchor in (anchor, {**anchor, 'draft': True}):
            with mock.patch.object(patch, 'api', side_effect=[[stable], {'sha': prior}, observed_anchor]) as api, \
                    mock.patch.object(patch.time, 'time', return_value=stamp), self.assertRaises(ValueError):
                patch.activation_check(self.root, '6.6.2-rc.1', self.head())
            self.assertEqual('releases/tags/v6.6.0', api.call_args.args[0])

    def test_actual_activation_and_recovery_shell_refuse_before_publication_post(self):
        """Execute the final workflow boundary with the real guard and offline gh."""
        import yaml
        self.assemble()
        repo = Path(__file__).resolve().parents[2]
        bin_dir = Path(self.temp.name) / 'bin'
        bin_dir.mkdir()
        gh = bin_dir / 'gh'
        gh.write_text('''#!/usr/bin/env python3
import json,os,pathlib,sys
args=sys.argv[1:]
if '-X' in args:
    assert args[args.index('-X')+1] == 'PATCH'
    pathlib.Path(os.environ['POST_MARKER']).write_text('publication attempted')
    print('{}')
else:
    assert args[0] == 'api'
    evidence=json.loads(pathlib.Path(os.environ['API_EVIDENCE']).read_text())
    if args[1] not in evidence: sys.exit(22)
    print(json.dumps(evidence[args[1]]))
''')
        gh.chmod(0o755)
        evidence = Path(self.temp.name) / 'api.json'
        marker = Path(self.temp.name) / 'post'
        now = __import__('time').time()
        _, stable = self.activation_evidence()
        stable['published_at'] = datetime.fromtimestamp(now - 86400, timezone.utc).isoformat()
        env = {**os.environ, 'PATH': str(bin_dir) + os.pathsep + os.environ['PATH'],
               'TAG': 'v6.6.1', 'EXPECTED_COMMIT': self.head(), 'RELEASE_ID': '123',
               'GITHUB_REPOSITORY': patch.REPOSITORY, 'POST_MARKER': str(marker),
               'release_json': str(Path(self.temp.name) / 'published.json'),
               'publish_payload': str(Path(self.temp.name) / 'payload.json'),
               'API_EVIDENCE': str(evidence)}
        for path, job_name, step_name in (('.github/workflows/create-release.yml', 'activate_release',
                                          'Publish the fully staged release'),
                                         ('.github/workflows/recover-release-activation.yml', 'recover_activation',
                                          'Commit the recovered activation')):
            workflow = yaml.load((repo / path).read_text(), Loader=yaml.BaseLoader)
            script = next(step['run'] for step in workflow['jobs'][job_name]['steps'] if step.get('name') == step_name)
            boundary = script.split('unset IMMUTABILITY_ADMIN_TOKEN\n', 1)[1].split('activated=true', 1)[0]
            self.assertIn('stable_patch.py activation-check', boundary)
            boundary = boundary.replace('python3 scripts/release_control/stable_patch.py',
                                        'python3 ' + str(repo / 'scripts/release_control/stable_patch.py'))
            boundary = boundary.replace('${{ github.repository }}', '${GITHUB_REPOSITORY}')
            for current in (stable, {**stable, 'tag_name': 'v6.7.0'}):
                marker.unlink(missing_ok=True)
                evidence.write_text(json.dumps({f'repos/{patch.REPOSITORY}/releases?per_page=100&page=1': [current],
                                                f'repos/{patch.REPOSITORY}/commits/v6.6.0': {'sha': self.base}}))
                result = subprocess.run(['bash', '-euc', boundary], cwd=self.root, env=env,
                                        capture_output=True, text=True)
                with self.subTest(workflow=path, current=current['tag_name']):
                    self.assertEqual(current is stable, result.returncode == 0, result.stderr)
                    self.assertEqual(current is stable, marker.exists())


class StablePatchGovernanceTest(unittest.TestCase):
    """Real patch targets must produce the independently required check context."""

    def workflow(self):
        import yaml
        repo = Path(__file__).resolve().parents[2]
        return yaml.load((repo / '.github/workflows/canonical-governance.yml').read_text(),
                         Loader=yaml.BaseLoader)

    def assert_admitted(self, workflow, version, event):
        branch = release_branch_for_version(version)
        self.assertEqual('release/v' + version.removesuffix('-rc.1'), branch)
        trigger = workflow['on'][event]
        # A paths filter would strand metadata-only promotion PRs even when
        # their target matches. The required job must not be renamed or skipped.
        self.assertFalse({'branches-ignore', 'paths', 'paths-ignore'} & trigger.keys())
        self.assertTrue(any(fnmatchcase(branch, pattern) for pattern in trigger['branches']))
        job = workflow['jobs']['governance']
        self.assertEqual('governance', job.get('name', 'governance'))
        self.assertNotIn('if', job)
        self.assertNotIn('needs', job)
        self.assertNotIn('continue-on-error', job)
        steps = '\n'.join(step.get('run', '') for step in job['steps'])
        self.assertIn('canonical_completion_guard.py', steps)
        self.assertIn('stable_patch_test.py', steps)

    def test_actual_governance_triggers_admit_candidate_and_promotion_targets(self):
        workflow = self.workflow()
        for version in ('6.6.1-rc.1', '6.6.1', '6.6.2-rc.1', '6.6.2'):
            for event in ('pull_request', 'push'):
                with self.subTest(version=version, event=event):
                    self.assert_admitted(workflow, version, event)
        # Extending branch admission must not import the private post-merge
        # credential-bearing workflow into an unreviewed preparation.
        self.assertEqual({'contents': 'read'}, workflow['permissions'])
        self.assertIn('main', workflow['on']['pull_request']['branches'])
        self.assertIn('main', workflow['on']['push']['branches'])

    def test_old_main_only_or_filtered_or_renamed_job_cannot_pass(self):
        for event in ('pull_request', 'push'):
            changes = ('old-main-only', 'path-filter', 'renamed', 'conditional', 'tolerant')
            for change in changes:
                with self.subTest(event=event, change=change):
                    workflow = self.workflow()
                    trigger, job = workflow['on'][event], workflow['jobs']['governance']
                    if change == 'old-main-only':
                        trigger['branches'] = ['main']
                    elif change == 'path-filter':
                        trigger['paths'] = ['scripts/**']
                    elif change == 'renamed':
                        job['name'] = 'patch-governance'
                    elif change == 'conditional':
                        job['if'] = "github.base_ref == 'main'"
                    else:
                        job['continue-on-error'] = 'true'
                    with self.assertRaises(AssertionError):
                        self.assert_admitted(workflow, '6.6.1-rc.1', event)


if __name__ == '__main__':
    unittest.main()
