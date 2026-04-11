#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';

const ROOT = process.cwd();

const REQUIRED_PAGE_HEADERS = new Map([
  ['src/pages/AIIntelligence.tsx', 'PageHeader'],
  ['src/pages/Ceph.tsx', 'PageHeader'],
  ['src/pages/Dashboard.tsx', 'PageHeader'],
  ['src/pages/Infrastructure.tsx', 'PageHeader'],
  ['src/pages/Operations.tsx', 'PageHeader'],
  ['src/pages/NotFound.tsx', 'PageHeader'],
  ['src/pages/PricingHandoff.tsx', 'PageHeader'],
  ['src/components/Settings/Settings.tsx', 'PageHeader'],
]);

const REQUIRED_OPERATIONS_WRAPPERS = new Map([
  ['src/components/Settings/DiagnosticsPanel.tsx', 'OperationsPanel'],
  ['src/components/Settings/ReportingPanel.tsx', 'OperationsPanel'],
  ['src/components/Settings/SystemLogsPanel.tsx', 'OperationsPanel'],
]);

const NON_VISUAL_PAGE_WRAPPERS = new Set([
  // Router wrapper; renders Recovery component and intentionally has no page chrome.
  'src/pages/RecoveryRoute.tsx',
]);

const SETTINGS_PANEL_SHIMS = new Set([]);

const HEADER_PRIMITIVES = ['PageHeader', 'SectionHeader', 'SettingsPanel', 'OperationsPanel'];

function readFileSafe(relPath) {
  const absPath = path.join(ROOT, relPath);
  if (!fs.existsSync(absPath)) return '';
  return fs.readFileSync(absPath, 'utf8');
}

function hasPrimitive(content, primitive) {
  return new RegExp(`<${primitive}\\b`).test(content);
}

function resolveImport(specifier, fromFile) {
  let basePath = null;
  if (specifier.startsWith('@/')) {
    basePath = path.join(ROOT, 'src', specifier.slice(2));
  } else if (specifier.startsWith('.')) {
    basePath = path.resolve(path.dirname(path.join(ROOT, fromFile)), specifier);
  }

  if (!basePath) {
    return null;
  }

  const candidates = [
    basePath,
    `${basePath}.tsx`,
    `${basePath}.ts`,
    path.join(basePath, 'index.tsx'),
    path.join(basePath, 'index.ts'),
  ];

  for (const candidate of candidates) {
    if (fs.existsSync(candidate) && fs.statSync(candidate).isFile()) {
      return path.relative(ROOT, candidate);
    }
  }

  return null;
}

function getImportedLocalFiles(relPath) {
  const content = readFileSafe(relPath);
  if (!content) {
    return [];
  }

  const imports = new Set();
  const importPattern = /from\s+['"]([^'"]+)['"]|import\(\s*['"]([^'"]+)['"]\s*\)/g;
  let match;
  while ((match = importPattern.exec(content)) !== null) {
    const specifier = match[1] ?? match[2];
    const resolved = resolveImport(specifier, relPath);
    if (resolved) {
      imports.add(resolved);
    }
  }

  return Array.from(imports);
}

function collectPrimitiveUsage(relPath, visited = new Set()) {
  if (visited.has(relPath)) {
    return new Set();
  }
  visited.add(relPath);

  const content = readFileSafe(relPath);
  const primitives = new Set(
    HEADER_PRIMITIVES.filter((primitive) => hasPrimitive(content, primitive)),
  );

  for (const importedFile of getImportedLocalFiles(relPath)) {
    for (const primitive of collectPrimitiveUsage(importedFile, visited)) {
      primitives.add(primitive);
    }
  }

  return primitives;
}

function listTopLevelPages() {
  const dir = path.join(ROOT, 'src/pages');
  return fs
    .readdirSync(dir, { withFileTypes: true })
    .filter((entry) => entry.isFile() && entry.name.endsWith('.tsx'))
    .map((entry) => `src/pages/${entry.name}`)
    .sort();
}

function listTopLevelSettingsPanels() {
  const registryFile = 'src/components/Settings/settingsPanelRegistryLoaders.ts';
  const content = readFileSafe(registryFile);
  const panels = new Set();
  const importPattern = /import\('\.\/([^']+)'\)/g;
  let match;
  while ((match = importPattern.exec(content)) !== null) {
    const moduleName = match[1];
    if (!moduleName.endsWith('Panel')) {
      continue;
    }
    panels.add(`src/components/Settings/${moduleName}.tsx`);
  }
  return Array.from(panels).sort();
}

const failures = [];

for (const [file, requiredPrimitive] of REQUIRED_PAGE_HEADERS.entries()) {
  const content = readFileSafe(file);
  if (!content) {
    failures.push(`${file}: missing file`);
    continue;
  }
  const primitives = collectPrimitiveUsage(file);
  if (!primitives.has(requiredPrimitive)) {
    failures.push(`${file}: must use <${requiredPrimitive}>`);
  }
}

for (const [file, requiredPrimitive] of REQUIRED_OPERATIONS_WRAPPERS.entries()) {
  const content = readFileSafe(file);
  if (!content) {
    failures.push(`${file}: missing file`);
    continue;
  }
  if (!hasPrimitive(content, requiredPrimitive)) {
    failures.push(`${file}: must use <${requiredPrimitive}>`);
  }
}

const pageInventory = [];
for (const pageFile of listTopLevelPages()) {
  const content = readFileSafe(pageFile);
  const primitives = Array.from(collectPrimitiveUsage(pageFile));
  const hasRawH1 = /<h1\b/.test(content);

  pageInventory.push({
    file: pageFile,
    primitives,
    hasRawH1,
  });

  if (NON_VISUAL_PAGE_WRAPPERS.has(pageFile)) {
    continue;
  }

  if (primitives.length === 0) {
    failures.push(`${pageFile}: must use at least one shared header primitive (${HEADER_PRIMITIVES.join(', ')})`);
  }

  if (hasRawH1) {
    failures.push(`${pageFile}: raw <h1> detected (use <PageHeader> instead)`);
  }
}

for (const panelFile of listTopLevelSettingsPanels()) {
  if (SETTINGS_PANEL_SHIMS.has(panelFile)) continue;

  const primitives = collectPrimitiveUsage(panelFile);
  const hasSharedWrapper = primitives.has('SettingsPanel') || primitives.has('OperationsPanel');

  if (!hasSharedWrapper) {
    failures.push(`${panelFile}: must use <SettingsPanel> or <OperationsPanel>`);
  }
}

console.log('Header Inventory (top-level pages):');
for (const page of pageInventory) {
  const primitiveSummary = page.primitives.length ? page.primitives.join(', ') : 'none';
  const h1Summary = page.hasRawH1 ? 'raw-h1' : 'no-raw-h1';
  console.log(`- ${page.file}: ${primitiveSummary} (${h1Summary})`);
}

if (failures.length > 0) {
  console.error('\nHeader Audit Failures:');
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log('\nHeader audit passed.');
