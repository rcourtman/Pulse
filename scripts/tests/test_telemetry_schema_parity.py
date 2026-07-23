import importlib.util
from pathlib import Path
import unittest


SCRIPT_PATH = Path(__file__).resolve().parents[1] / "check_telemetry_schema_parity.py"
SPEC = importlib.util.spec_from_file_location("check_telemetry_schema_parity", SCRIPT_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class TelemetrySchemaParityTest(unittest.TestCase):
    def test_matching_contract(self):
        public = '''
type Ping struct {
    SchemaVersion int `json:"schema_version"`
    Active bool `json:"active"`
}
'''
        receiver = '''
var ping struct {
    SchemaVersion int `json:"schema_version"`
    Active bool `json:"active"`
    LicenseTier string `json:"license_tier"`
    APITokens int `json:"api_tokens"`
}
'''
        frontend = '''
export interface TelemetryPingPreview {
    schema_version: number;
    active: boolean;
}
'''
        self.assertEqual(MODULE.parity_errors(public, receiver, frontend), [])

    def test_missing_and_mismatched_fields_fail(self):
        public = '''
type Ping struct {
    SchemaVersion int `json:"schema_version"`
    Active bool `json:"active"`
    Count int `json:"count"`
}
'''
        receiver = '''
var ping struct {
    SchemaVersion string `json:"schema_version"`
    Active int `json:"active"`
    Extra bool `json:"extra"`
}
'''
        frontend = '''
export interface TelemetryPingPreview {
    schema_version: string;
    active: boolean;
    unexpected: number;
}
'''
        errors = MODULE.parity_errors(public, receiver, frontend)
        self.assertTrue(any("count" in error for error in errors))
        self.assertTrue(any("extra" in error for error in errors))
        self.assertTrue(any("schema_version" in error and "active" in error for error in errors))
        self.assertTrue(any("frontend preview missing" in error and "count" in error for error in errors))
        self.assertTrue(any("frontend preview fields absent" in error and "unexpected" in error for error in errors))
        self.assertTrue(any("frontend field type mismatches" in error and "schema_version" in error for error in errors))


if __name__ == "__main__":
    unittest.main()
