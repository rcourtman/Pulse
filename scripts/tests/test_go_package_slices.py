#!/usr/bin/env python3
"""Include the runtime worker's contract controls in ordinary smoke discovery."""

import importlib.util
from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location(
    "go_package_slice_tests", ROOT / "scripts/release_control/run_go_package_slice_test.py")
TESTS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(TESTS)


def load_tests(loader, tests, pattern):
    return loader.loadTestsFromModule(TESTS)


if __name__ == "__main__":
    unittest.main()
