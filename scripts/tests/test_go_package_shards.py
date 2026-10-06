"""Whole-package balancing must never trade exhaustive CI for faster checks."""

import importlib.util
from pathlib import Path
import random
import subprocess
import tempfile
import unittest
from decimal import Decimal


ROOT = Path(__file__).resolve().parents[2]
SELECTOR = ROOT / ".github/scripts/select-go-package-shard.py"
spec = importlib.util.spec_from_file_location("package_shards", SELECTOR)
selector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(selector)


class PackageShardsTest(unittest.TestCase):
    def cli(self, text, weights="DEFAULT_WEIGHT 1\n", count=2, index=0):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "weights.txt"
            path.write_text(weights, encoding="utf-8")
            return subprocess.run(
                ["python3", str(SELECTOR), str(path), str(count), str(index)],
                input=text, text=True, capture_output=True,
            )

    def test_measured_heavy_packages_are_separated(self):
        # The previous odd/even split put both slow packages on shard 1.
        packages = ["example/a", "example/b", "example/c", "example/d"]
        weights = {"example/a": Decimal(100), "example/c": Decimal(80)}
        assigned = selector.assign(packages, weights, Decimal(1), 2)
        self.assertNotEqual(assigned["example/a"], assigned["example/c"])
        costs = [sum(weights.get(p, Decimal(1)) for p in packages if assigned[p] == i) for i in range(2)]
        self.assertEqual(costs, [Decimal(100), Decimal(82)])
        self.assertEqual(sum(costs), Decimal(182))

    def test_exhaustive_nonempty_deterministic_assignment_under_churn(self):
        randomizer = random.Random(60106)
        for count in range(1, 9):
            for size in (count, count + 1, count * 17):
                with self.subTest(count=count, size=size):
                    packages = [f"example/p{i}" for i in range(size)]
                    randomizer.shuffle(packages)
                    weights = {p: Decimal(randomizer.randint(1, 500)) for p in packages[::3]}
                    weights["example/deleted"] = Decimal(9999)
                    assigned = selector.assign(packages, weights, Decimal(1), count)
                    self.assertEqual(set(assigned), set(packages))
                    self.assertEqual(set(assigned.values()), set(range(count)))
                    self.assertEqual(assigned, selector.assign(list(reversed(packages)), weights, Decimal(1), count))

    def test_cli_keeps_source_order_and_new_unmeasured_packages(self):
        packages = ["example/new", "example/z", "example/a", "example/b", "example/renamed"]
        weights = "# stale names are not coverage\nDEFAULT_WEIGHT 2.5\nexample/deleted 100\nexample/z 20\n"
        selected = []
        for index in range(2):
            result = self.cli("\n".join(packages) + "\n", weights, index=index)
            self.assertEqual(result.returncode, 0, result.stderr)
            names = result.stdout.splitlines()
            self.assertTrue(names)
            self.assertEqual(names, [p for p in packages if p in names])
            selected.extend(names)
        self.assertCountEqual(selected, packages)

    def test_equal_weights_and_decimal_ties_are_stable(self):
        packages = ["example/d", "example/c", "example/b", "example/a"]
        assigned = selector.assign(packages, {}, Decimal("0.1"), 2)
        self.assertEqual(assigned, {"example/a": 0, "example/b": 1, "example/c": 0, "example/d": 1})

    def test_invalid_weights_fail_before_printing_any_package(self):
        for weights in ("example/a", "example/a nope", "example/a NaN", "example/a Infinity", "example/a -1", "example/a 0", "example/a 1e99999999", "example/a 1\nexample/a 2", "DEFAULT_WEIGHT 1\nDEFAULT_WEIGHT 2", "-option 1"):
            with self.subTest(weights=weights):
                result = self.cli("example/a\nexample/b\n", weights)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(result.stdout, "")
                self.assertIn("package shard selection failed:", result.stderr)

    def test_invalid_source_or_shard_fail_closed(self):
        for text, count, index in (("", 2, 0), ("example/a\n", 2, 0), ("example/a\nexample/a\n", 2, 0), ("-option\nexample/a\n", 2, 0), ("example/a extra\nexample/b\n", 2, 0), ("example/a\nexample/b\n", 0, 0), ("example/a\nexample/b\n", 2, -1), ("example/a\nexample/b\n", 2, 2)):
            with self.subTest(text=text, count=count, index=index):
                result = self.cli(text, count=count, index=index)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(result.stdout, "")

    def test_missing_weights_cannot_silently_fall_back(self):
        with tempfile.TemporaryDirectory() as directory:
            result = subprocess.run(["python3", str(SELECTOR), str(Path(directory) / "absent"), "2", "0"], input="example/a\nexample/b\n", text=True, capture_output=True)
        self.assertEqual(result.returncode, 2)
        self.assertEqual(result.stdout, "")

    def test_committed_weights_are_valid_and_positive(self):
        weights, default = selector.read_weights(ROOT / ".github/scripts/go-package-test-seconds.txt")
        self.assertGreater(default, 0)
        self.assertGreater(len(weights), 0)
        self.assertTrue(all(value > 0 for value in weights.values()))


if __name__ == "__main__":
    unittest.main()
