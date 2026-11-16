import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

import { afterEach, describe, expect, it } from 'vitest';

const auditPath = path.join(process.cwd(), 'scripts', 'form-label-audit.mjs');
const temporaryRoots = [];

const runAudit = (source) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-form-label-audit-'));
  temporaryRoots.push(root);
  fs.mkdirSync(path.join(root, 'src', 'nested'), { recursive: true });
  fs.writeFileSync(path.join(root, 'src', 'nested', 'Fixture.tsx'), source);
  return spawnSync(process.execPath, [auditPath], { cwd: root, encoding: 'utf8' });
};

afterEach(() => {
  while (temporaryRoots.length) {
    fs.rmSync(temporaryRoots.pop(), { recursive: true, force: true });
  }
  document.body.replaceChildren();
});

describe('form label audit', () => {
  it.each(['wrapped', 'targeted'])(
    'rejects a %s hidden-only field that the native label cannot control',
    (association) => {
      // Exercise the platform association too, not only the audit's wording.
      document.body.innerHTML =
        association === 'wrapped'
          ? '<label>User ID<input type="hidden"></label>'
          : '<label for="user">User ID</label><input type="hidden" id="user">';
      expect(document.querySelector('label').control).toBeNull();
      const source =
        association === 'wrapped'
          ? 'export const Field = () => <label>User ID<input type="hidden" /></label>;'
          : 'export const Field = () => <><label for="user">User ID</label><input type="hidden" id="user" /></>;';

      const result = runAudit(source);
      expect(result.status).toBe(1);
      expect(result.stderr).toContain('src/nested/Fixture.tsx:1:');
      expect(result.stderr).toContain('hidden inputs are not labelable');
    },
  );

  it.each(['"hidden"', '"HiDdEn"', "{'hidden'}", '{"hidden"}', '{`hidden`}', "{('hidden')}"])(
    'recognizes a static hidden type %s in both native association forms',
    (type) => {
      const result = runAudit(`export const Field = () => <>
        <label>User ID<input type=${type} /></label>
        <label htmlFor="user">User ID</label><input id="user" type=${type}></input>
      </>;`);

      expect(result.status).toBe(1);
      expect(result.stderr.match(/hidden inputs are not labelable/g)).toHaveLength(2);
    },
  );

  it.each(['button', 'input', 'meter', 'output', 'progress', 'select', 'textarea'])(
    'keeps accepting wrapped and targeted native %s controls',
    (tag) => {
      const result = runAudit(`export const Field = () => <>
        <label>Value<${tag} /></label>
        <label for="value">Value</label><${tag} id="value" />
      </>;`);

      expect(result.status).toBe(0);
      expect(result.stdout).toContain('Form label audit passed (1 TSX files checked).');
      expect(result.stderr).toBe('');
    },
  );

  it('finds a visible control after an unlabelable hidden descendant', () => {
    document.body.innerHTML =
      '<label>User ID<input type="hidden"><span><input type="text"></span></label>';
    expect(document.querySelector('label').control).toBe(
      document.querySelector('input[type=text]'),
    );
    const result = runAudit(`export const Field = () => (
      <label>User ID<input type="hidden" /><span><input type="text" /></span></label>
    );`);

    expect(result.status).toBe(0);
  });

  it('does not use an unrelated visible input to validate a hidden explicit target', () => {
    const result = runAudit(`export const Field = () => <>
      <label for="secret">User ID<input type="text" /></label>
      <input id="secret" type="hidden" />
    </>;`);

    expect(result.status).toBe(1);
  });

  it('rejects a hidden type that overrides a preceding spread', () => {
    const result = runAudit(
      'export const Field = (props) => <label>User ID<input {...props} type="hidden" /></label>;',
    );

    expect(result.status).toBe(1);
  });

  it.each([
    'type="text"',
    'type=" hidden "',
    'type={inputType}',
    'type="hidden" {...props}',
    'type',
    '',
  ])('does not invent a hidden runtime type from %s', (attributes) => {
    const result = runAudit(
      `export const Field = () => <label>User ID<input ${attributes} /></label>;`,
    );

    expect(result.status).toBe(0);
  });

  it('still rejects labels with no control or a missing target', () => {
    const result = runAudit(`export const Field = () => <>
      <label>Missing control</label><label for="absent">Missing target</label>
      <input id="other" />
    </>;`);

    expect(result.status).toBe(1);
    expect(result.stderr.match(/label must target/g)).toHaveLength(2);
  });
});
