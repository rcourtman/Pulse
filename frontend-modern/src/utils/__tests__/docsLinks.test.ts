import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  API_TOKEN_SCOPES_DOC_URL,
  CONFIGURATION_DOC_URL,
  PRIVACY_DOC_URL,
  PROXY_AUTH_DOC_URL,
  README_DOC_URL,
  SECURITY_DOC_URL,
  SHIPPED_DOCS_ROOT,
  getShippedDocUrl,
} from '@/utils/docsLinks';
import apiAccessPanelSource from '@/components/Settings/APIAccessPanel.tsx?raw';
import apiTokenManagerModelSource from '@/components/Settings/apiTokenManagerModel.ts?raw';
import securityOverviewPanelSource from '@/components/Settings/SecurityOverviewPanel.tsx?raw';
import securityWarningSource from '@/components/SecurityWarning.tsx?raw';

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
    expect(CONFIGURATION_DOC_URL).toBe('/docs/CONFIGURATION.md');
    expect(PROXY_AUTH_DOC_URL).toBe('/docs/PROXY_AUTH.md');
    expect(SECURITY_DOC_URL).toBe('/docs/SECURITY.md');
    expect(API_TOKEN_SCOPES_DOC_URL).toBe('/docs/CONFIGURATION.md');
  });

  it('keeps shipped docs content synced with repo docs', () => {
    const docPairs = [
      { source: path.join(repoRoot, 'docs', 'README.md'), target: 'README.md' },
      { source: path.join(repoRoot, 'docs', 'PRIVACY.md'), target: 'PRIVACY.md' },
      { source: path.join(repoRoot, 'docs', 'CONFIGURATION.md'), target: 'CONFIGURATION.md' },
      { source: path.join(repoRoot, 'docs', 'PROXY_AUTH.md'), target: 'PROXY_AUTH.md' },
      { source: path.join(repoRoot, 'SECURITY.md'), target: 'SECURITY.md' },
    ];

    for (const { source, target } of docPairs) {
      const rootDoc = readFileSync(source, 'utf8');
      const publicDoc = readFileSync(path.join(frontendRoot, 'public', 'docs', target), 'utf8');
      expect(publicDoc).toBe(rootDoc);
      expect(publicDoc).not.toContain('https://github.com/rcourtman/Pulse/blob/main/');
    }
  });

  it('routes runtime docs links through shipped local docs instead of GitHub main', () => {
    expect(apiAccessPanelSource).toContain('API_TOKEN_SCOPES_DOC_URL');
    expect(apiAccessPanelSource).not.toContain('https://github.com/rcourtman/Pulse/blob/main/docs/');
    expect(apiTokenManagerModelSource).toContain("from '@/utils/docsLinks'");
    expect(apiTokenManagerModelSource).toContain('SHIPPED_API_TOKEN_SCOPES_DOC_URL');
    expect(apiTokenManagerModelSource).toContain('export const API_TOKEN_SCOPES_DOC_URL =');
    expect(apiTokenManagerModelSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/docs/',
    );
    expect(securityOverviewPanelSource).toContain('PROXY_AUTH_DOC_URL');
    expect(securityOverviewPanelSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/docs/',
    );
    expect(securityWarningSource).toContain('SECURITY_DOC_URL');
    expect(securityWarningSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/docs/',
    );
  });
});
