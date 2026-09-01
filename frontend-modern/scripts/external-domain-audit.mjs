#!/usr/bin/env node
// Blocks fabricated Pulse-branded domains from entering published surfaces.
//
// Context: commit 524f42cc28 invented security@pulseapp.io and docs.pulseapp.io
// (pulseapp.io was never a Pulse domain); the address shipped as the published
// security contact for ~10 months and bounced every disclosure. A follow-up
// audit on 2026-09-01 found https://pulse.app claimed as the OpenRouter
// attribution referer. This audit is the offline guard for that class: any
// domain-shaped token containing "pulse" that is not on the owned/placeholder
// allowlist fails the push. It cannot judge ownership of non-Pulse-branded
// domains; the periodic link audit covers those.
import fs from 'node:fs';
import path from 'node:path';

const ROOT = process.cwd();
const REPO_ROOT = path.resolve(ROOT, '..');

// Real public suffixes a fabricated brand domain would plausibly use. Tokens
// ending in anything else (.role, .local, .example, .sh script names, Go
// identifiers) are not treated as domains.
const PUBLIC_TLDS = new Set([
  'com', 'net', 'org', 'io', 'app', 'dev', 'pro', 'cloud', 'ai', 'me',
  'co', 'eu', 'de', 'es', 'fr', 'uk', 'us', 'xyz', 'info', 'tech', 'online',
  'site', 'blog',
]);

function isAllowedHost(host) {
  if (host === 'pulserelay.pro' || host.endsWith('.pulserelay.pro')) return true;
  if (host === '1mk.app' || host.endsWith('.1mk.app')) return true;
  // Cloudflare Pages previews of the landing site.
  if (host.endsWith('.pages.dev') && host.includes('pulserelay-landing')) return true;
  const labels = host.split('.');
  // Documentation placeholders.
  if (labels.includes('example') || labels.some((l) => l.includes('yourdomain'))) return true;
  if (host.startsWith('your-') || host.startsWith('yourname.')) return true;
  return false;
}

const SCAN_ROOT_FILES = ['SECURITY.md', 'README.md', 'CONTRIBUTING.md', 'TERMS.md'];
const SCAN_DIRS = [
  { dir: 'docs', extensions: null },
  { dir: 'frontend-modern/public/docs', extensions: null },
  { dir: 'frontend-modern/src', extensions: /\.(ts|tsx)$/ },
  { dir: 'internal', extensions: /\.go$/ },
  { dir: 'cmd', extensions: /\.go$/ },
  { dir: 'pkg', extensions: /\.go$/ },
];
const SKIP_DIR_NAMES = new Set(['node_modules', '__tests__', 'testdata', '.git']);
const SKIP_FILE_PATTERNS = [/_test\.go$/, /\.test\.(ts|tsx)$/, /\.(png|jpg|jpeg|gif|webp|svg|ico|woff2?|mp4|pdf|zip|gz)$/i];
const DOMAIN_TOKEN = /[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+/gi;

const findings = [];

function toRelative(absPath) {
  return path.relative(REPO_ROOT, absPath).replaceAll(path.sep, '/');
}

function collectFiles(directory, extensions) {
  const files = [];
  let entries;
  try {
    entries = fs.readdirSync(directory, { withFileTypes: true });
  } catch {
    return files;
  }
  for (const entry of entries) {
    if (SKIP_DIR_NAMES.has(entry.name)) continue;
    const fullPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...collectFiles(fullPath, extensions));
      continue;
    }
    if (!entry.isFile()) continue;
    if (extensions && !extensions.test(entry.name)) continue;
    if (SKIP_FILE_PATTERNS.some((p) => p.test(entry.name))) continue;
    files.push(fullPath);
  }
  return files;
}

function inspectFile(absPath) {
  let content;
  try {
    content = fs.readFileSync(absPath, 'utf8');
  } catch {
    return;
  }
  if (content.includes('\u0000')) return;
  const lines = content.split('\n');
  for (let i = 0; i < lines.length; i++) {
    for (const match of lines[i].matchAll(DOMAIN_TOKEN)) {
      // Capitalized single-label .app tokens are macOS/iOS bundle names
      // (Pulse.app), not domains; a fabricated domain reads lowercase.
      if (/^[A-Z][A-Za-z0-9-]*\.app$/.test(match[0])) continue;
      const host = match[0].toLowerCase();
      if (!host.includes('pulse')) continue;
      const labels = host.split('.');
      if (!PUBLIC_TLDS.has(labels[labels.length - 1])) continue;
      if (isAllowedHost(host)) continue;
      findings.push({ file: toRelative(absPath), line: i + 1, host });
    }
  }
}

const targets = SCAN_ROOT_FILES.map((f) => path.join(REPO_ROOT, f)).filter((f) => fs.existsSync(f));
for (const { dir, extensions } of SCAN_DIRS) {
  targets.push(...collectFiles(path.join(REPO_ROOT, dir), extensions));
}
for (const file of targets) inspectFile(file);

if (findings.length > 0) {
  console.error(
    'External domain audit failed. Pulse-branded domains must be pulserelay.pro or 1mk.app;',
  );
  console.error(
    'anything else is fabricated (see the pulseapp.io incident). Fix the reference or, for a',
  );
  console.error('legitimate new Pulse-owned domain, add it to isAllowedHost with a comment.');
  for (const finding of findings) {
    console.error(`- ${finding.file}:${finding.line}: ${finding.host}`);
  }
  process.exit(1);
}

console.log('External domain audit passed with no unrecognized Pulse-branded domains.');
