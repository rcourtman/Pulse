#!/usr/bin/env node
// A security override is not blanket compatibility approval for a new Solid.
// Check the locked runtime edges before npm ci can hide them behind overrides.
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const defaultLock = new URL('../frontend-modern/package-lock.json', import.meta.url);
const stableVersion = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/;

function versionParts(value) {
  const match = typeof value === 'string' && stableVersion.exec(value);
  const parts = match ? match.slice(1).map(Number) : null;
  return parts?.every(Number.isSafeInteger) ? parts : null;
}

function resolvedDependency(packages, consumerPath, dependency) {
  let parent = consumerPath;
  while (true) {
    const nested = `${parent}/node_modules/${dependency}`;
    if (Object.hasOwn(packages, nested)) return { path: nested, entry: packages[nested] };
    const boundary = parent.lastIndexOf('/node_modules/');
    if (boundary < 0) break;
    parent = parent.slice(0, boundary);
  }
  const root = `node_modules/${dependency}`;
  return Object.hasOwn(packages, root) ? { path: root, entry: packages[root] } : null;
}

export function serializationGraphErrors(lock) {
  const packages = lock?.packages;
  if (lock?.lockfileVersion !== 3 || !packages || typeof packages !== 'object' || Array.isArray(packages)) {
    return ['Expected an npm v3 lock with a complete packages map.'];
  }
  const consumers = Object.entries(packages).filter(([path]) =>
    path === 'node_modules/solid-js' || path.endsWith('/node_modules/solid-js'),
  );
  if (!consumers.some(([path]) => path === 'node_modules/solid-js')) {
    return ['The frontend lock must contain its direct Solid runtime.'];
  }
  const errors = [];
  for (const [path, consumer] of consumers) {
    if (!versionParts(consumer?.version)) {
      errors.push('Solid must have an exact stable locked version.');
      continue;
    }
    for (const dependency of ['seroval', 'seroval-plugins']) {
      const range = consumer.dependencies?.[dependency];
      // These runtime edges currently use full stable tilde ranges. A changed
      // range format needs review rather than an invented semver interpretation.
      const floor = typeof range === 'string' && range.startsWith('~') && versionParts(range.slice(1));
      if (!floor) {
        errors.push(`Solid ${consumer.version}: review the new ${dependency} requirement (expected ~major.minor.patch).`);
        continue;
      }
      const resolved = resolvedDependency(packages, path, dependency);
      const installed = versionParts(resolved?.entry?.version);
      if (!installed) {
        errors.push(`Solid ${consumer.version}: ${dependency} must resolve to an exact stable locked version.`);
        continue;
      }
      const compatible = installed[0] === floor[0] && installed[1] === floor[1] && installed[2] >= floor[2];
      // 795e534517 qualified this one security override with actual cyclic,
      // typed-array and plugin consumers. Preserve it, but do not apply that
      // old proof to a new consumer, dependency, declared range or nested copy.
      const qualifiedOverride = path === 'node_modules/solid-js' &&
        consumer.version === '1.9.15' && dependency === 'seroval' &&
        range === '~1.5.4' && resolved.path === 'node_modules/seroval' &&
        resolved.entry.version === '1.6.3';
      if (!compatible && !qualifiedOverride) {
        errors.push(`Solid ${consumer.version} requires ${dependency} ${range}, but resolves ${resolved.entry.version}. Update the coupled graph and retain consumer compatibility proof; an npm override is not that proof.`);
      }
    }
  }
  return errors;
}

export function main(args = process.argv.slice(2)) {
  if (args.length !== 0 && (args.length !== 2 || args[0] !== '--lockfile')) {
    console.error('Usage: node scripts/check-frontend-serialization.mjs [--lockfile FILE]');
    return 2;
  }
  let lock;
  try {
    lock = JSON.parse(readFileSync(args.length ? args[1] : defaultLock, 'utf8'));
  } catch {
    console.error('Frontend serialization graph: lock read/JSON parse failed.');
    return 2;
  }
  const errors = serializationGraphErrors(lock);
  for (const error of errors) console.error(`Frontend serialization graph: ${error}`);
  if (errors.length) return 1;
  console.log('Frontend serialization graph: Solid runtime edges match, or use the exact qualified security override.');
  return 0;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exitCode = main();
}
