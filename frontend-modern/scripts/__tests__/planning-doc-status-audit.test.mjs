import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

import { afterEach, describe, expect, it } from 'vitest';

const auditPath = path.join(process.cwd(), 'scripts', 'planning-doc-status-audit.mjs');
const temporaryRoots = [];

function writeFile(file, content) {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, content);
}

function git(repoRoot, ...args) {
  const result = spawnSync('git', ['-C', repoRoot, ...args], { encoding: 'utf8' });
  if (result.status !== 0) {
    throw new Error(`git ${args.join(' ')} failed: ${result.stderr}`);
  }
}

function runAudit(frontendRoot) {
  return spawnSync(process.execPath, [auditPath], { cwd: frontendRoot, encoding: 'utf8' });
}

afterEach(() => {
  while (temporaryRoots.length > 0) {
    fs.rmSync(temporaryRoots.pop(), { force: true, recursive: true });
  }
});

describe('planning document status audit', () => {
  it('checks tracked planning documents and ignores workspace-only files', () => {
    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-planning-doc-audit-'));
    temporaryRoots.push(repoRoot);
    const frontendRoot = path.join(repoRoot, 'frontend-modern');
    const trackedPlan = path.join(repoRoot, 'docs', 'TRACKED_PLAN.md');
    const ignoredContract = path.join(repoRoot, 'docs', 'architecture', 'IGNORED_CONTRACT.md');
    const subsystemContract = path.join(
      repoRoot,
      'docs',
      'release-control',
      'v6',
      'internal',
      'subsystems',
      'IGNORED_CONTRACT.md',
    );

    fs.mkdirSync(frontendRoot, { recursive: true });
    writeFile(path.join(repoRoot, '.gitignore'), 'docs/architecture/\n');
    writeFile(trackedPlan, '# Tracked plan\n');
    writeFile(ignoredContract, '# Ignored local contract\n');
    writeFile(subsystemContract, '# Separately governed subsystem contract\n');
    git(repoRoot, 'init', '--quiet');
    git(repoRoot, 'add', '.gitignore', 'docs/TRACKED_PLAN.md', 'docs/release-control');

    const failing = runAudit(frontendRoot);
    expect(failing.status).toBe(1);
    expect(failing.stderr).toContain('docs/TRACKED_PLAN.md');
    expect(failing.stderr).not.toContain('IGNORED_CONTRACT.md');

    writeFile(trackedPlan, '# Tracked plan\n\nStatus: PARKED. Last reviewed 2026-09-01.\n');
    const passing = runAudit(frontendRoot);
    expect(passing.status).toBe(0);
    expect(passing.stdout).toContain('planning-doc-status-audit: ok');
  });
});
