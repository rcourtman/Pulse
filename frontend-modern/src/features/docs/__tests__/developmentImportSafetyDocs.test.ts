import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const guide = readFileSync(path.join(root, 'frontend-modern/public/docs/CONTRIBUTING.md'), 'utf8');

function article() {
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(guide, 'CONTRIBUTING');
  return element;
}

const prose = () => article().textContent!.replace(/\s+/g, ' ');

describe('shipped development import safety guidance', () => {
  it('ships the complete current contributor guide, including key refusal recovery', () => {
    expect(guide).toBe(readFileSync(path.join(root, 'CONTRIBUTING.md'), 'utf8'));
    const text = prose();
    expect(text).toContain('requires GNU realpath and reads installed files only');
    expect(text).toContain('different installed/development keys, stop the import');
    expect(text).toContain('Preserve both data sets and identify the matching key independently');
    expect(text).toContain('do not delete ciphertext or pick the newest backup');
    expect(text).toContain('Use a separate empty development copy');
  });

  it('keeps staged files private without promising full or atomic recovery', () => {
    const text = prose();
    expect(text).toContain('leaves development sessions, CSRF state, OIDC, local .env, history and alerts alone');
    expect(text).toContain('stages reads before per-file replacement');
    expect(text).toContain('keeps overwritten files privately in .config-import.*/previous');
    expect(text).toContain('not a full-state backup or a consistent snapshot of a running installation');
    expect(text).toContain('keep the snapshot and remaining staged files');
    expect(text).toContain('inspect the partial import privately before starting anything');
    expect(text).toContain('The helper starts no service');
  });

  it('preserves the existing explicit-selection and failure-preserving cleanup guidance', () => {
    const text = prose();
    expect(text).toContain('Keep both installed-data selectors false for community reproduction');
    expect(text).toContain('Neither is a workaround for a startup refusal');
    expect(text).toContain('do not sandbox local environment files or establish host isolation');
    expect(text).toContain('Neither command resets Pulse');
    expect(text).toContain('cleanup stops before removing anything');
    expect(text).toContain('Removal failures are reported without using sudo');
    expect(text).toContain('Cleanup does not stop processes or restore anything an earlier cleanup deleted');
    expect(article().querySelector('#cleaning-development-build-output')).not.toBeNull();
    expect(article().querySelector('a[href="#hot-reload-dev-loop"]')).not.toBeNull();
  });
});
