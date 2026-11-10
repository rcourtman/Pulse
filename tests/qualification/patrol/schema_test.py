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

    def response(self):
        return copy.deepcopy(self.catalogue["response.host-disk-one-folder.json"])

    def test_response_scenarios_need_their_rubric_and_engagement(self):
        for field in ("outcome", "engagement"):
            with self.subTest(field=field):
                manifest = self.response()
                manifest.pop(field)
                self.assertFalse(self.validator.is_valid(manifest))
        manifest = self.response()
        manifest["outcome"].pop("decisive_terms")
        self.assertFalse(self.validator.is_valid(manifest))
        manifest = self.response()
        manifest["investigation"] = {"min_evidence_calls": 1, "required_summary_terms": ["disk"], "require_completed_status": True}
        self.assertFalse(self.validator.is_valid(manifest))

    def test_quiet_outcome_carries_no_terms(self):
        manifest = copy.deepcopy(self.catalogue["response.finished-job-stays-quiet.json"])
        self.validator.validate(manifest)
        manifest["outcome"]["decisive_terms"] = [["anything"]]
        self.assertFalse(self.validator.is_valid(manifest))

    def test_rubric_and_engagement_are_limited_to_response_scenarios(self):
        for field in ("outcome", "engagement"):
            with self.subTest(field=field):
                manifest = copy.deepcopy(self.catalogue["watch.healthy-mixed.json"])
                manifest[field] = self.response()[field]
                self.assertFalse(self.validator.is_valid(manifest))

    def test_host_filesystem_resource_is_only_a_bounded_mount_path(self):
        def host_resource(manifest):
            return next(r for r in manifest["resources"] if r["kind"] == "host_filesystem")

        for mount_path in ("/", "/var/lib/docker", "/mnt", "/mnt/nvr/recordings", "/home/user/disk"):
            with self.subTest(mount_path=mount_path):
                manifest = self.response()
                host_resource(manifest)["mount_path"] = mount_path
                self.assertFalse(self.validator.is_valid(manifest))
        manifest = self.response()
        host_resource(manifest).pop("mount_path")
        self.assertFalse(self.validator.is_valid(manifest))
        manifest = self.response()
        host_resource(manifest)["command"] = ["/bin/sh"]
        self.assertFalse(self.validator.is_valid(manifest))


if __name__ == "__main__":
    unittest.main()
