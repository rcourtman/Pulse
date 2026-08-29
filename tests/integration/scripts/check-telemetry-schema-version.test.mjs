import assert from "node:assert/strict";
import test from "node:test";

import {
  githubErrorAnnotation,
  readVersion,
  telemetrySchemaVersionErrors,
} from "./check-telemetry-schema-version.mjs";

test("accepts a stable E2E expectation matching the public payload", () => {
  assert.deepEqual(
    telemetrySchemaVersionErrors(
      "const ( TelemetrySchemaVersion = 15 )",
      "const EXPECTED_TELEMETRY_SCHEMA_VERSION = 15;",
    ),
    [],
  );
});

test("reports both versions when the stable E2E expectation drifts", () => {
  assert.deepEqual(
    telemetrySchemaVersionErrors(
      "TelemetrySchemaVersion = 16",
      "const EXPECTED_TELEMETRY_SCHEMA_VERSION = 15;",
    ),
    ["telemetry schema version drift: public payload is 16, stable E2E expects 15"],
  );
});

test("rejects missing or ambiguous version declarations", () => {
  assert.throws(
    () => readVersion("", /VERSION=(\d+)/, "fixture"),
    /fixture must declare exactly one schema version; found 0/,
  );
  assert.throws(
    () => readVersion("VERSION=1 VERSION=2", /VERSION=(\d+)/, "fixture"),
    /fixture must declare exactly one schema version; found 2/,
  );
});

test("emits a source-linked GitHub annotation with escaped data", () => {
  assert.equal(
    githubErrorAnnotation("public is 16\nE2E is 15%"),
    "::error file=tests/integration/tests/19-telemetry-disclosure.spec.ts," +
      "title=Telemetry schema version parity::public is 16%0AE2E is 15%25",
  );
});
