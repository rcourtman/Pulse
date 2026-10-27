"""Whole merge results and exact controls, with real Git but no network or publishing."""
from copy import deepcopy
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

import stable_patch as patch


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

    def test_dirty_checkout_is_not_discarded(self):
        self.write('untracked', 'do not discard')
        with self.assertRaisesRegex(ValueError, 'clean disposable'):
            self.assemble()
        self.assertEqual('do not discard', (self.root / 'untracked').read_text())


if __name__ == '__main__':
    unittest.main()
