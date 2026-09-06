"""Validate the public authoring contract with a complete JSON Schema engine."""

import copy
import json
from pathlib import Path
import unittest

from jsonschema.validators import validator_for


class PublishedSchemaTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        root = Path(__file__).resolve().parent
        schema = json.loads((root / "patrol.qual.schema.json").read_text())
        validator_type = validator_for(schema)
        validator_type.check_schema(schema)
        cls.validator = validator_type(schema)
        cls.catalogue = {
            path.name: json.loads(path.read_text())
            for path in sorted((root / "scenarios").glob("*.json"))
        }

    def test_complete_catalogue(self):
        self.assertTrue(self.catalogue)
        for name, manifest in self.catalogue.items():
            with self.subTest(manifest=name):
                self.validator.validate(manifest)

    def investigation(self):
        return copy.deepcopy(self.catalogue["investigation.docker-storage-pressure.json"])

    def test_either_summary_expectation_is_supported(self):
        for field, value in (
            ("required_summary_terms", ["storage"]),
            ("required_summary_term_groups", [["storage", "filesystem"]]),
        ):
            with self.subTest(field=field):
                manifest = self.investigation()
                spec = manifest["investigation"]
                spec.pop("required_summary_terms", None)
                spec.pop("required_summary_term_groups", None)
                spec[field] = value
                self.validator.validate(manifest)

    def test_missing_summary_expectations_are_rejected(self):
        manifest = self.investigation()
        spec = manifest["investigation"]
        spec.pop("required_summary_terms", None)
        spec.pop("required_summary_term_groups", None)
        self.assertFalse(self.validator.is_valid(manifest))

    def test_empty_summary_groups_are_rejected(self):
        for groups in ([], [[]], [[""]]):
            with self.subTest(groups=groups):
                manifest = self.investigation()
                manifest["investigation"]["required_summary_term_groups"] = groups
                self.assertFalse(self.validator.is_valid(manifest))

    def test_summary_groups_do_not_bypass_evidence_requirement(self):
        manifest = self.investigation()
        spec = manifest["investigation"]
        spec.pop("min_evidence_calls", None)
        spec.pop("min_evidence_ids", None)
        self.assertFalse(self.validator.is_valid(manifest))


if __name__ == "__main__":
    unittest.main()
