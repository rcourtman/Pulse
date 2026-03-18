#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';

const ROOT = process.cwd();
const SRC_DIR = path.join(ROOT, 'src');
const INDEX_HTML = path.join(ROOT, 'index.html');

const IGNORE_DIRS = new Set([
  'node_modules',
  'dist',
  'coverage',
  '.git',
]);

const THEME_OWNER_ALLOWLIST = new Set([
  'src/utils/theme.ts',
  'index.html',
]);

const findings = [];

function toRelative(absPath) {
  return path.relative(ROOT, absPath).replaceAll(path.sep, '/');
}

function lineForIndex(content, index) {
  let line = 1;
  for (let i = 0; i < index; i += 1) {
    if (content[i] === '\n') line += 1;
  }
  return line;
}

function addFinding({ file, index, rule, message }) {
  const line = lineForIndex(file.content, index);
  findings.push({
    file: file.relativePath,
    line,
    rule,
    message,
  });
}

function collectFiles(dir) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  const files = [];

  for (const entry of entries) {
    if (IGNORE_DIRS.has(entry.name)) continue;
    const fullPath = path.join(dir, entry.name);

    if (entry.isDirectory()) {
      files.push(...collectFiles(fullPath));
      continue;
    }

    if (!entry.isFile()) continue;
    if (!/\.(ts|tsx|js|jsx|mjs|cjs)$/.test(entry.name)) continue;
    files.push(fullPath);
  }

  return files;
}

function analyzeFile(absPath) {
  const relativePath = toRelative(absPath);
  const content = fs.readFileSync(absPath, 'utf8');
  const file = { relativePath, content };
  const isThemeOwner = THEME_OWNER_ALLOWLIST.has(relativePath);
  const isTestFile = /\/__tests__\//.test(relativePath) || /\.test\.(ts|tsx|js|jsx)$/.test(relativePath);

  if (!isThemeOwner && !isTestFile) {
    const darkClassPattern = /document\.documentElement\.classList\.(?:add|remove|toggle)\(\s*['"]dark['"]/g;
    for (const match of content.matchAll(darkClassPattern)) {
      addFinding({
        file,
        index: match.index ?? 0,
        rule: 'theme-owner/root-dark-class',
        message: "Only theme owners may mutate document.documentElement.classList('dark'). Use src/utils/theme.ts helpers.",
      });
    }

    const directThemeStoragePattern = /localStorage\.(?:getItem|setItem|removeItem)\(\s*['"](pulseThemePreference|darkMode|pulse_dark_mode)['"]/g;
    for (const match of content.matchAll(directThemeStoragePattern)) {
      addFinding({
        file,
        index: match.index ?? 0,
        rule: 'theme-owner/storage-keys',
        message: 'Only theme owners may read/write theme storage keys directly. Use src/utils/theme.ts.',
      });
    }

    const storageKeyConstantPattern = /\bSTORAGE_KEYS\.(?:THEME_PREFERENCE|DARK_MODE)\b/g;
    for (const match of content.matchAll(storageKeyConstantPattern)) {
      addFinding({
        file,
        index: match.index ?? 0,
        rule: 'theme-owner/storage-constants',
        message: 'Do not consume STORAGE_KEYS.THEME_PREFERENCE/DARK_MODE directly outside src/utils/theme.ts.',
      });
    }
  }

  // Full-screen surfaces should use semantic backgrounds so light/dark always resolve.
  const classLiteralPatterns = [
    /class(?:Name)?\s*=\s*["']([^"']*min-h-screen[^"']*)["']/g,
    /class(?:Name)?\s*=\s*{\s*`([^`]*min-h-screen[^`]*)`\s*}/g,
  ];

  for (const pattern of classLiteralPatterns) {
    for (const match of content.matchAll(pattern)) {
      const classString = match[1] ?? '';
      const hasSemanticBackground = /\bbg-(?:base|surface|surface-alt|surface-hover)\b/.test(classString);
      const hasHardcodedBackground = /\bbg-(?:white|black|slate-\d{2,3}|gray-\d{2,3}|zinc-\d{2,3}|neutral-\d{2,3}|stone-\d{2,3}|red-\d{2,3}|orange-\d{2,3}|amber-\d{2,3}|yellow-\d{2,3}|lime-\d{2,3}|green-\d{2,3}|emerald-\d{2,3}|teal-\d{2,3}|cyan-\d{2,3}|sky-\d{2,3}|blue-\d{2,3}|indigo-\d{2,3}|violet-\d{2,3}|purple-\d{2,3}|fuchsia-\d{2,3}|pink-\d{2,3}|rose-\d{2,3})\b/.test(classString);

      if (!hasSemanticBackground && hasHardcodedBackground) {
        addFinding({
          file,
          index: match.index ?? 0,
          rule: 'theme-shell/semantic-background',
          message: 'min-h-screen wrappers must use semantic backgrounds (bg-base/bg-surface/bg-surface-alt).',
        });
      }
    }
  }
}

const filesToAnalyze = [
  ...collectFiles(SRC_DIR),
  INDEX_HTML,
];

for (const filePath of filesToAnalyze) {
  if (!fs.existsSync(filePath)) continue;
  analyzeFile(filePath);
}

if (findings.length === 0) {
  console.log('Theme audit passed with no findings.');
  process.exit(0);
}

console.error('Theme audit findings:');
for (const finding of findings) {
  console.error(`- ${finding.file}:${finding.line} [${finding.rule}] ${finding.message}`);
}

const byRule = new Map();
for (const finding of findings) {
  byRule.set(finding.rule, (byRule.get(finding.rule) ?? 0) + 1);
}

console.error('\nSummary by rule:');
for (const [rule, count] of byRule.entries()) {
  console.error(`- ${rule}: ${count}`);
}

process.exit(1);
