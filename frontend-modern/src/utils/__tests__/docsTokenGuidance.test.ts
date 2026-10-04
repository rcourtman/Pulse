import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { API_SCOPE_OPTIONS } from '@/constants/apiScopes';
import { getAPITokenScopePresets } from '@/components/Settings/apiTokenManagerModel';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const configuration = readFileSync(path.join(repoRoot, 'docs/CONFIGURATION.md'), 'utf8');
const tokens = configuration.split('## 🔑 API Tokens')[1]?.split('\n## TrueNAS')[0] ?? '';
const normalized = tokens.replace(/\s+/g, ' ');

function tableRows(section: string) {
  return section
    .split('\n')
    .filter((line) => line.startsWith('|') && !/^\|\s*-/.test(line))
    .slice(1)
    .map((line) =>
      line
        .split('|')
        .slice(1, -1)
        .map((cell) => cell.trim()),
    );
}

describe('shipped API Access guidance', () => {
  it('names the existing token controls rather than the retired navigation', () => {
    expect(normalized).toContain('Settings → API Access');
    expect(normalized).not.toContain('Security → API Tokens');
    for (const control of ['New token', 'Create token', 'Quick preset', 'Generate']) {
      expect(normalized).toContain(control);
    }
    expect(normalized).toContain('cannot create or revoke tokens');
  });

  it('documents every UI scope without inventing capabilities', () => {
    const scopeSection = tokens.split('### Token Scopes')[1]?.split('### Presets')[0] ?? '';
    const scopes = tableRows(scopeSection).map(([scope]) => scope.match(/`([^`]+)`/)?.[1]);
    expect(scopes.sort()).toEqual(['*', ...API_SCOPE_OPTIONS.map((scope) => scope.value)].sort());
  });

  it('matches names and exact least-privilege scope sets to the actual preset model', () => {
    const presetSection = tokens.split('### Presets')[1]?.split('### Kiosk Mode')[0] ?? '';
    const documented = tableRows(presetSection).map(([label, scopes]) => ({
      label: label.replace(/\*\*/g, ''),
      scopes: [...scopes.matchAll(/`([^`]+)`/g)].map((match) => match[1]),
    }));
    expect(documented).toEqual(
      getAPITokenScopePresets().map(({ label, scopes }) => ({ label, scopes })),
    );
    expect(normalized).toContain('Reporting is not remote execution');
    expect(normalized).toContain('not to fix a missing reading');
    expect(normalized).toContain('separate collector and action-runner');
  });

  it('distinguishes backend-required Patrol scopes and wildcard authority from presets', () => {
    expect(normalized).toContain('its scopes come from the current Patrol requirements');
    expect(normalized).toContain('legacy `*` wildcard, not a least-privilege preset');
    expect(normalized).toContain("Hiding controls does not reduce a token's permissions");
  });

  it('keeps credential rotation separate from identity deletion and exposure response', () => {
    const rotation =
      tokens.split('### Replace or revoke a token')[1]?.split('### Token Scopes')[0] ?? '';
    const text = rotation.replace(/\s+/g, ' ');
    expect(text).toContain('fresh authenticated result before revoking the old token');
    expect(text).toContain('Last-used metadata alone does not account for every consumer');
    expect(text).toContain('Do not reinstall an agent or delete its saved identity');
    expect(text).toContain('revoke it promptly');
    expect(text).toContain('even if that interrupts monitoring or a display');
    expect(text).toContain('Do not leave a leaked token active');
    expect(text).toContain('[agent retargeting](UNIFIED_AGENT.md#moving-pulse-to-a-new-address)');
    expect(text).toContain('[private header-file procedure](API.md#api-token-recommended)');
  });

  it('does not present a hidden dashboard or cleaned URL as a credential boundary', () => {
    const kiosk = tokens.split('### Kiosk Mode')[1] ?? '';
    const text = kiosk.replace(/\s+/g, ' ');
    expect(text).toContain('Kiosk / Monitoring');
    expect(text).toContain('only `monitoring:read`');
    expect(text).toContain('Magic Kiosk Link');
    expect(text).toContain('Copy Link');
    expect(text).toContain('HTTPS');
    expect(text).toContain('no administrator session');
    expect(text).toContain('bearer credential');
    expect(text).toContain('it does not authenticate a browser or change');
    expect(text).toContain('initial request already carried the token');
    expect(text).toContain('proxy/server logs');
    expect(text).toContain('session storage');
    expect(text).toContain('Do not rely on this session surviving');
    expect(text).not.toContain('?token=YOUR_TOKEN_HERE');
    expect(text).not.toContain('avoid cookie persistence issues');
  });

  it('ships the same scope guidance the UI links to', () => {
    expect(
      readFileSync(path.join(repoRoot, 'frontend-modern/public/docs/CONFIGURATION.md'), 'utf8'),
    ).toBe(configuration);
  });
});
