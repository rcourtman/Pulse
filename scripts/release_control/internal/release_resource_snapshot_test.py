import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
spec = importlib.util.spec_from_file_location('resource_snapshot', ROOT / 'scripts/release-resource-snapshot.py')
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class ResourceSnapshotTest(unittest.TestCase):
    def test_ancestors_and_missing_counters(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            proc, root = base / 'proc', base / 'cgroup'
            (proc / 'self').mkdir(parents=True)
            (root / 'parent/leaf').mkdir(parents=True)
            (proc / 'self/cgroup').write_text('0::/parent/leaf\n')
            (proc / 'self/mountinfo').write_text(f'1 0 0:1 / {root} rw - cgroup2 cgroup rw\n')
            (root / 'parent/cpu.max').write_text('600000 100000\n')
            result = module.cgroups(proc, root)
            self.assertEqual(len(result), 3)
            self.assertEqual(result[1]['files']['cpu.max'], '600000 100000')
            self.assertEqual(result[0]['files']['cpu.stat'], {'unavailable_errno': 2})
            self.assertNotIn('parent', json.dumps(result))
            (proc / 'self/cgroup').write_text('0::/../../outside\n')
            self.assertIn('unavailable', module.cgroups(proc, root))
            (proc / 'self/cgroup').write_text('0::/parent/leaf\n')
            (proc / 'self/mountinfo').write_text('')
            self.assertIn('unavailable', module.cgroups(proc, root))

    def test_environment_allowlist_and_numeric_filter(self):
        with patch.dict(os.environ, {'GITHUB_TOKEN': 'synthetic-secret', 'GOGC': 'synthetic-secret', 'GOMEMLIMIT': '512MiB', 'GOMAXPROCS': '2'}):
            result = module.snapshot()
            self.assertNotIn('synthetic-secret', json.dumps(result))
            self.assertEqual(result['runtime_knobs']['GOMEMLIMIT'], '512MiB')
            self.assertEqual(result['runtime_knobs']['GOMAXPROCS'], '2')



if __name__ == '__main__':
    unittest.main()
