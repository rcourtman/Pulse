import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  PRIVACY_DOC_URL,
  README_DOC_URL,
  SHIPPED_DOCS_ROOT,
  getShippedDocUrl,
} from '@/utils/docsLinks';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const frontendRoot = path.resolve(__dirname, '..', '..', '..');
const repoRoot = path.resolve(frontendRoot, '..');

describe('docsLinks', () => {
  it('returns canonical shipped doc URLs', () => {
    expect(SHIPPED_DOCS_ROOT).toBe('/docs');
    expect(getShippedDocUrl('PRIVACY.md')).toBe('/docs/PRIVACY.md');
    expect(PRIVACY_DOC_URL).toBe('/docs/PRIVACY.md');
    expect(README_DOC_URL).toBe('/docs/README.md');
  });

  it('keeps shipped privacy and docs content synced with repo docs', () => {
    const rootPrivacy = readFileSync(path.join(repoRoot, 'docs', 'PRIVACY.md'), 'utf8');
    const publicPrivacy = readFileSync(
      path.join(frontendRoot, 'public', 'docs', 'PRIVACY.md'),
      'utf8',
    );
    const rootReadme = readFileSync(path.join(repoRoot, 'docs', 'README.md'), 'utf8');
    const publicReadme = readFileSync(path.join(frontendRoot, 'public', 'docs', 'README.md'), 'utf8');

    expect(publicPrivacy).toBe(rootPrivacy);
    expect(publicReadme).toBe(rootReadme);
  });
});
