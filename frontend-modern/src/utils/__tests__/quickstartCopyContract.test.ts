import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../../../../');
const readRepoFile = (relativePath: string) => readFileSync(resolve(repoRoot, relativePath), 'utf-8');

describe('quickstart copy contract', () => {
  it('removes anonymous Community quickstart claims from public docs', () => {
    const readme = readRepoFile('README.md');
    const pulsePro = readRepoFile('docs/PULSE_PRO.md');
    const pricingSpec = readRepoFile('docs/architecture/v6-pricing-and-tiering.md');

    expect(readme).not.toContain('every new workspace gets 25 Patrol quickstart runs');
    expect(readme).not.toContain('every new workspace gets 25 Patrol runs');
    expect(pulsePro).not.toContain('new workspaces get 25 Patrol quickstart runs');
    expect(pulsePro).not.toContain('Patrol quickstart for new workspaces: 25 Patrol runs with no API key.');
    expect(pricingSpec).not.toContain('Every new workspace gets 25');
  });

  it('describes quickstart as activation-gated Patrol-only support', () => {
    const readme = readRepoFile('README.md');
    const pulsePro = readRepoFile('docs/PULSE_PRO.md');
    const pricingSpec = readRepoFile('docs/architecture/v6-pricing-and-tiering.md');

    expect(readme).toContain('Activated or trial-backed installs can use 25 Patrol quickstart runs');
    expect(readme).toMatch(
      /Unactivated Community installs should activate, start\s+a trial, or use BYOK\./,
    );
    expect(pulsePro).toContain(
      'Activated or trial-backed installs can use 25 Patrol quickstart runs with no API key for first-run activation.',
    );
    expect(pricingSpec).toMatch(
      /Activated or trial-backed\s+installs with the server-verified installation identity get 25 hosted Patrol runs/,
    );
    expect(pricingSpec).toMatch(/it is not a general hosted\s+chat entitlement/);
  });
});
