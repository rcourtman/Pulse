import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { extname, relative, resolve } from 'node:path';

const root = resolve(
  process.env.PULSE_SHARED_TEMPLATE_AUDIT_ROOT || new URL('..', import.meta.url).pathname,
);
const registryPath = 'scripts/shared-template-registry.json';

const fileContentsCache = new Map();
const fileExistsCache = new Map();
const scopeTreeCache = new Map();
const collectFilesCache = new Map();

const read = (path) => {
  let content = fileContentsCache.get(path);
  if (content === undefined) {
    content = readFileSync(resolve(root, path), 'utf8');
    fileContentsCache.set(path, content);
  }
  return content;
};
const readJson = (path) => JSON.parse(read(path));
const fileExists = (path) => {
  let exists = fileExistsCache.get(path);
  if (exists === undefined) {
    exists = existsSync(resolve(root, path));
    fileExistsCache.set(path, exists);
  }
  return exists;
};
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

const listScopeFiles = (scopeRoot) => {
  const cached = scopeTreeCache.get(scopeRoot);
  if (cached !== undefined) return cached;

  const files = [];
  const visitDirectory = (directoryPath) => {
    for (const entry of readdirSync(directoryPath, { withFileTypes: true })) {
      const entryPath = resolve(directoryPath, entry.name);
      if (entry.isDirectory()) {
        visitDirectory(entryPath);
      } else if (entry.isFile()) {
        files.push(toRegistryPath(entryPath));
      } else if (entry.isSymbolicLink()) {
        visitPath(entryPath);
      } else {
        files.push(toRegistryPath(entryPath));
      }
    }
  };
  const visitPath = (absolutePath) => {
    const stat = statSync(absolutePath);
    if (stat.isDirectory()) {
      visitDirectory(absolutePath);
      return;
    }
    files.push(toRegistryPath(absolutePath));
  };

  visitPath(scopeRoot);
  files.sort();
  scopeTreeCache.set(scopeRoot, files);
  return files;
};

const collectFiles = (scope, extensions) => {
  const scopeRoot = resolve(root, scope);
  if (!fileExists(scope)) {
    failures.push(`pattern guard scope ${scope} does not exist`);
    return [];
  }

  const cacheKey = `${scopeRoot}\0${extensions.join(',')}`;
  const cached = collectFilesCache.get(cacheKey);
  if (cached !== undefined) return cached;

  const extensionSet = new Set(extensions);
  const files = listScopeFiles(scopeRoot).filter((file) => extensionSet.has(extname(file)));
  collectFilesCache.set(cacheKey, files);
  return files;
};

const collectScopedFiles = (scopes, extensions) => {
  if (scopes.length === 1) {
    return collectFiles(scopes[0], extensions);
  }
  return [...new Set(scopes.flatMap((scope) => collectFiles(scope, extensions)))].sort();
};

const orderPatternsForMatch = (patterns) =>
  patterns.length <= 1 ? patterns : [...patterns].sort((a, b) => b.length - a.length);

const getAllowedPath = (entry) => (typeof entry === 'string' ? entry : entry?.path);
const filterGuardFiles = (files, guard) => {
  const pathIncludes = Array.isArray(guard.pathIncludes) ? guard.pathIncludes : [];
  const pathExcludes = Array.isArray(guard.pathExcludes) ? guard.pathExcludes : [];
  return files.filter(
    (file) =>
      pathIncludes.every((fragment) => file.includes(fragment)) &&
      !pathExcludes.some((fragment) => file.includes(fragment)),
  );
};

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
  const ignoredEntries = guard.ignoredPaths ?? [];
  const ignoredPaths = new Set(ignoredEntries.map(getAllowedPath).filter(Boolean));
  const orderedPatterns = orderPatternsForMatch(patterns);
  const files = filterGuardFiles(collectScopedFiles(scopes, extensions), guard);

  for (const file of files) {
    if (file === guard.canonical.path) continue;
    if (ignoredPaths.has(file)) continue;

    const source = read(file);
    const matches = orderedPatterns.every((pattern) => source.includes(pattern));
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

  for (const ignoredPath of ignoredPaths) {
    if (!fileExists(ignoredPath)) {
      failures.push(`${guard.id}: ignored path ${ignoredPath} does not exist`);
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
  const requiredAnyPatterns = Array.isArray(guard.requiredAnyPatterns)
    ? guard.requiredAnyPatterns
    : [];
  if (requiredPatterns.length === 0 && requiredAnyPatterns.length === 0) {
    failures.push(`${guard.id}: missing requiredPatterns or requiredAnyPatterns`);
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
  const ignoredEntries = guard.ignoredPaths ?? [];
  const ignoredPaths = new Set(ignoredEntries.map(getAllowedPath).filter(Boolean));
  const orderedTriggerPatterns = orderPatternsForMatch(triggerPatterns);
  const files = filterGuardFiles(collectScopedFiles(scopes, extensions), guard);

  for (const file of files) {
    if (file === guard.canonical.path) continue;
    if (ignoredPaths.has(file)) continue;

    const source = read(file);
    const triggered = orderedTriggerPatterns.every((pattern) => source.includes(pattern));
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

    if (
      requiredAnyPatterns.length > 0 &&
      !requiredAnyPatterns.some((pattern) => source.includes(pattern))
    ) {
      failures.push(
        `${guard.id}: ${file} matches ${JSON.stringify(
          triggerPatterns,
        )} but does not compose one of ${JSON.stringify(requiredAnyPatterns)}`,
      );
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

  for (const ignoredPath of ignoredPaths) {
    if (!fileExists(ignoredPath)) {
      failures.push(`${guard.id}: ignored path ${ignoredPath} does not exist`);
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
