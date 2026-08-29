#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath, pathToFileURL } from "node:url";

const GO_SCHEMA_PATTERN = /\bTelemetrySchemaVersion\s*=\s*(\d+)\b/;
const E2E_SCHEMA_PATTERN =
  /\bEXPECTED_TELEMETRY_SCHEMA_VERSION\s*=\s*(\d+)\b/;

export function readVersion(source, pattern, label) {
  const matches = [...source.matchAll(new RegExp(pattern.source, "g"))];
  if (matches.length !== 1) {
    throw new Error(
      `${label} must declare exactly one schema version; found ${matches.length}`,
    );
  }
  return Number.parseInt(matches[0][1], 10);
}

export function telemetrySchemaVersionErrors(goSource, e2eSource) {
  const publicVersion = readVersion(
    goSource,
    GO_SCHEMA_PATTERN,
    "internal/telemetry/telemetry.go",
  );
  const e2eVersion = readVersion(
    e2eSource,
    E2E_SCHEMA_PATTERN,
    "tests/integration/tests/19-telemetry-disclosure.spec.ts",
  );
  if (publicVersion === e2eVersion) return [];
  return [
    `telemetry schema version drift: public payload is ${publicVersion}, ` +
      `stable E2E expects ${e2eVersion}`,
  ];
}

export function githubErrorAnnotation(message) {
  const escaped = message
    .replaceAll("%", "%25")
    .replaceAll("\r", "%0D")
    .replaceAll("\n", "%0A");
  return (
    "::error file=tests/integration/tests/19-telemetry-disclosure.spec.ts," +
    `title=Telemetry schema version parity::${escaped}`
  );
}

export function main() {
  const integrationRoot = path.resolve(
    fileURLToPath(new URL("..", import.meta.url)),
  );
  const repoRoot = path.resolve(integrationRoot, "../..");
  const goPath = path.join(
    repoRoot,
    "internal",
    "telemetry",
    "telemetry.go",
  );
  const e2ePath = path.join(
    integrationRoot,
    "tests",
    "19-telemetry-disclosure.spec.ts",
  );

  let errors;
  try {
    errors = telemetrySchemaVersionErrors(
      fs.readFileSync(goPath, "utf8"),
      fs.readFileSync(e2ePath, "utf8"),
    );
  } catch (error) {
    errors = [error instanceof Error ? error.message : String(error)];
  }
  if (errors.length) {
    console.error("Telemetry schema version validation failed:");
    for (const error of errors) {
      console.error(`- ${error}`);
      console.error(githubErrorAnnotation(error));
    }
    return 1;
  }

  console.log("Telemetry schema version validation passed");
  return 0;
}

if (
  process.argv[1] &&
  pathToFileURL(path.resolve(process.argv[1])).href === import.meta.url
) {
  process.exitCode = main();
}
