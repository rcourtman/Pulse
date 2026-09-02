import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

import { afterEach, describe, expect, it } from 'vitest';

const auditPath = path.join(process.cwd(), 'scripts', 'table-row-a11y-audit.mjs');
const temporaryRoots = [];

const writeSource = (frontendRoot, source) => {
  const sourcePath = path.join(frontendRoot, 'src', 'Fixture.tsx');
  fs.mkdirSync(path.dirname(sourcePath), { recursive: true });
  fs.writeFileSync(sourcePath, source);
};

const runAudit = (frontendRoot) =>
  spawnSync(process.execPath, [auditPath], { cwd: frontendRoot, encoding: 'utf8' });

const makeFrontendRoot = () => {
  const frontendRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-table-row-a11y-'));
  temporaryRoots.push(frontendRoot);
  return frontendRoot;
};

afterEach(() => {
  while (temporaryRoots.length > 0) {
    fs.rmSync(temporaryRoots.pop(), { force: true, recursive: true });
  }
});

describe('table row accessibility audit', () => {
  it('rejects native and shared table rows in the tab sequence', () => {
    const frontendRoot = makeFrontendRoot();
    writeSource(
      frontendRoot,
      `export const Fixture = () => (
        <table><tbody>
          <tr tabindex="0"><td>Native row</td></tr>
          <TableRow tabIndex={0}><TableCell>Shared row</TableCell></TableRow>
        </tbody></table>
      );`,
    );

    const result = runAudit(frontendRoot);
    expect(result.status).toBe(1);
    expect(result.stderr).toContain('src/Fixture.tsx:3');
    expect(result.stderr).toContain('src/Fixture.tsx:4');
  });

  it('accepts a static row with a native disclosure button', () => {
    const frontendRoot = makeFrontendRoot();
    writeSource(
      frontendRoot,
      `export const Fixture = () => (
        <table><tbody><tr><td>
          <button type="button" aria-expanded="false" aria-controls="details">Details</button>
        </td></tr></tbody></table>
      );`,
    );

    const result = runAudit(frontendRoot);
    expect(result.status).toBe(0);
    expect(result.stdout).toContain('Table row accessibility audit passed');
  });
});
