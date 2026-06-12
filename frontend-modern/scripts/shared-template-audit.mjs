import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { extname, relative, resolve } from 'node:path';

const root = resolve(new URL('..', import.meta.url).pathname);
const registryPath = 'scripts/shared-template-registry.json';

const read = (path) => readFileSync(resolve(root, path), 'utf8');
const readJson = (path) => JSON.parse(read(path));
const fileExists = (path) => existsSync(resolve(root, path));
const toRegistryPath = (path) => relative(root, path).split('\\').join('/');

const registry = readJson(registryPath);
const rules = Array.isArray(registry.rules) ? registry.rules : [];
const patternGuards = Array.isArray(registry.patternGuards) ? registry.patternGuards : [];
const requiredPatternGuards = Array.isArray(registry.requiredPatternGuards)
  ? registry.requiredPatternGuards
  : [];

const failures = [];

const getCanonicalExport = (canonical) =>
  canonical.export ??
  canonical.path
    .split('/')
    .at(-1)
    ?.replace(/\.[^.]+$/, '');

const verifyCanonicalExport = (id, canonical) => {
  if (!canonical?.path) {
    failures.push(`${id}: missing canonical.path`);
    return undefined;
  }

  if (!fileExists(canonical.path)) {
    failures.push(`${id}: canonical template ${canonical.path} does not exist`);
    return undefined;
  }

  const canonicalSource = read(canonical.path);
  const canonicalExport = getCanonicalExport(canonical);
  if (canonicalExport && !canonicalSource.includes(canonicalExport)) {
    failures.push(`${id}: canonical template ${canonical.path} must expose ${canonicalExport}`);
  }
  return canonicalExport;
};

const collectFiles = (scope, extensions) => {
  const scopeRoot = resolve(root, scope);
  if (!existsSync(scopeRoot)) {
    failures.push(`pattern guard scope ${scope} does not exist`);
    return [];
  }

  const files = [];
  const visit = (absolutePath) => {
    const stat = statSync(absolutePath);
    if (stat.isDirectory()) {
      for (const entry of readdirSync(absolutePath)) {
        visit(resolve(absolutePath, entry));
      }
      return;
    }

    if (extensions.includes(extname(absolutePath))) {
      files.push(toRegistryPath(absolutePath));
    }
  };

  visit(scopeRoot);
  return files;
};

const getAllowedPath = (entry) => (typeof entry === 'string' ? entry : entry?.path);

for (const rule of rules) {
  if (!rule.id) failures.push('registry rule is missing id');
  if (!rule.summary) failures.push(`${rule.id}: missing summary`);
  if (!rule.category) failures.push(`${rule.id}: missing category`);
  if (!rule.canonical?.path) failures.push(`${rule.id}: missing canonical.path`);

  if (!rule.canonical?.path) continue;

  const canonicalExport = verifyCanonicalExport(rule.id, rule.canonical);

  for (const consumer of rule.requiredConsumers ?? []) {
    const source = read(consumer.path);
    if (canonicalExport && !source.includes(canonicalExport)) {
      failures.push(`${rule.id}: ${consumer.path} must compose ${canonicalExport}`);
    }
  }

  for (const check of rule.forbiddenPatterns ?? []) {
    const source = read(check.path);
    for (const pattern of check.patterns) {
      if (source.includes(pattern)) {
        failures.push(`${rule.id}: ${check.path} must not contain ${JSON.stringify(pattern)}`);
      }
    }
  }
}

for (const guard of patternGuards) {
  if (!guard.id) failures.push('pattern guard is missing id');
  if (!guard.summary) failures.push(`${guard.id}: missing summary`);
  if (!guard.category) failures.push(`${guard.id}: missing category`);
  if (!guard.canonical?.path) failures.push(`${guard.id}: missing canonical.path`);

  if (!guard.id || !guard.canonical?.path) continue;

  verifyCanonicalExport(guard.id, guard.canonical);

  const patterns = Array.isArray(guard.allPatterns) ? guard.allPatterns : [];
  if (patterns.length === 0) {
    failures.push(`${guard.id}: missing allPatterns`);
    continue;
  }

  const scopes = Array.isArray(guard.scopes) ? guard.scopes : [];
  if (scopes.length === 0) {
    failures.push(`${guard.id}: missing scopes`);
    continue;
  }

  const extensions = guard.extensions ?? ['.ts', '.tsx'];
  const allowedEntries = guard.allowedPaths ?? [];
  const allowedPaths = new Set(allowedEntries.map(getAllowedPath).filter(Boolean));
  const files = [...new Set(scopes.flatMap((scope) => collectFiles(scope, extensions)))].sort();

  for (const file of files) {
    if (file === guard.canonical.path) continue;

    const source = read(file);
    const matches = patterns.every((pattern) => source.includes(pattern));
    if (!matches) continue;

    if (allowedPaths.has(file)) continue;

    failures.push(
      `${guard.id}: ${file} recreates the shared template; compose ${guard.canonical.path} instead`,
    );
  }

  for (const allowedPath of allowedPaths) {
    if (!fileExists(allowedPath)) {
      failures.push(`${guard.id}: allowed path ${allowedPath} does not exist`);
      continue;
    }

    if (allowedPath === guard.canonical.path) {
      failures.push(`${guard.id}: canonical path ${allowedPath} must not be allowlisted`);
      continue;
    }

    const source = read(allowedPath);
    const stillMatches = patterns.every((pattern) => source.includes(pattern));
    if (!stillMatches) {
      failures.push(`${guard.id}: allowed path ${allowedPath} is stale; remove it from the guard`);
    }
  }
}

for (const guard of requiredPatternGuards) {
  if (!guard.id) failures.push('required pattern guard is missing id');
  if (!guard.summary) failures.push(`${guard.id}: missing summary`);
  if (!guard.category) failures.push(`${guard.id}: missing category`);
  if (!guard.canonical?.path) failures.push(`${guard.id}: missing canonical.path`);

  if (!guard.id || !guard.canonical?.path) continue;

  verifyCanonicalExport(guard.id, guard.canonical);

  const triggerPatterns = Array.isArray(guard.triggerPatterns) ? guard.triggerPatterns : [];
  if (triggerPatterns.length === 0) {
    failures.push(`${guard.id}: missing triggerPatterns`);
    continue;
  }

  const requiredPatterns = Array.isArray(guard.requiredPatterns) ? guard.requiredPatterns : [];
  if (requiredPatterns.length === 0) {
    failures.push(`${guard.id}: missing requiredPatterns`);
    continue;
  }

  const scopes = Array.isArray(guard.scopes) ? guard.scopes : [];
  if (scopes.length === 0) {
    failures.push(`${guard.id}: missing scopes`);
    continue;
  }

  const extensions = guard.extensions ?? ['.ts', '.tsx'];
  const allowedEntries = guard.allowedPaths ?? [];
  const allowedPaths = new Set(allowedEntries.map(getAllowedPath).filter(Boolean));
  const files = [...new Set(scopes.flatMap((scope) => collectFiles(scope, extensions)))].sort();

  for (const file of files) {
    if (file === guard.canonical.path) continue;

    const source = read(file);
    const triggered = triggerPatterns.every((pattern) => source.includes(pattern));
    if (!triggered) continue;

    if (allowedPaths.has(file)) continue;

    for (const pattern of requiredPatterns) {
      if (!source.includes(pattern)) {
        failures.push(
          `${guard.id}: ${file} matches ${JSON.stringify(
            triggerPatterns,
          )} but does not compose ${guard.canonical.path}`,
        );
        break;
      }
    }
  }

  for (const allowedPath of allowedPaths) {
    if (!fileExists(allowedPath)) {
      failures.push(`${guard.id}: allowed path ${allowedPath} does not exist`);
      continue;
    }

    if (allowedPath === guard.canonical.path) {
      failures.push(`${guard.id}: canonical path ${allowedPath} must not be allowlisted`);
      continue;
    }

    const source = read(allowedPath);
    const stillTriggered = triggerPatterns.every((pattern) => source.includes(pattern));
    if (!stillTriggered) {
      failures.push(`${guard.id}: allowed path ${allowedPath} is stale; remove it from the guard`);
    }
  }
}

if (failures.length > 0) {
  console.error('Shared template audit failed:');
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log(
  `Shared template audit passed (${rules.length} rule${rules.length === 1 ? '' : 's'}, ${
    patternGuards.length
  } pattern guard${patternGuards.length === 1 ? '' : 's'}, ${
    requiredPatternGuards.length
  } required pattern guard${requiredPatternGuards.length === 1 ? '' : 's'} from ${relative(
    process.cwd(),
    resolve(root, registryPath),
  )})`,
);
