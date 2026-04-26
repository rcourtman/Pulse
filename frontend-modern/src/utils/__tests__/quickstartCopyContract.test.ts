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

    expect(readme).toContain(
      'optional managed quickstart runs are available only after explicit Pulse Account activation',
    );
    expect(readme).toContain('if you explicitly activate through Pulse Account, Pulse can use 25');
    expect(pulsePro).toContain(
      'Optional Patrol quickstart after explicit Pulse Account activation: 25 Patrol runs with no API key on a server-verified install.',
    );
    expect(pricingSpec).toMatch(
      /optional Patrol-only quickstart allowance after explicit Pulse Account\s+activation/,
    );
    expect(pricingSpec).toMatch(/it is not a general\s+hosted chat entitlement/);
  });

  it('keeps hosted quickstart privacy copy aligned with resource-policy redaction', () => {
    const privacy = readRepoFile('docs/PRIVACY.md');
    const publicPrivacy = readRepoFile('frontend-modern/public/docs/PRIVACY.md');
    const aiSettingsDialog = readRepoFile(
      'frontend-modern/src/components/Settings/AISettingsDialogs.tsx',
    );

    for (const copy of [privacy, publicPrivacy]) {
      expect(copy).toContain('resource-policy redaction is applied before the Quickstart request');
      expect(copy).toContain('requests transit Pulse infrastructure');
      expect(copy).toContain('To keep prompts off Pulse infrastructure entirely, use a BYOK provider');
    }
    expect(aiSettingsDialog).toContain('Hosted quickstart routes policy-redacted prompts');
  });
});
