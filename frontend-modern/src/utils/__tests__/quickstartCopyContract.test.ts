import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../../../../');
const readRepoFile = (relativePath: string) => readFileSync(resolve(repoRoot, relativePath), 'utf-8');

describe('quickstart copy contract', () => {
  it('keeps self-hosted public docs free of hosted quickstart claims', () => {
    const readme = readRepoFile('README.md');
    const pulsePro = readRepoFile('docs/PULSE_PRO.md');
    const ai = readRepoFile('docs/AI.md');
    const privacy = readRepoFile('docs/PRIVACY.md');
    const security = readRepoFile('SECURITY.md');
    const publicPrivacy = readRepoFile('frontend-modern/public/docs/PRIVACY.md');
    const publicSecurity = readRepoFile('frontend-modern/public/docs/SECURITY.md');
    const pricingSpec = readRepoFile('docs/architecture/v6-pricing-and-tiering.md');
    const aiSettingsDialog = readRepoFile(
      'frontend-modern/src/components/Settings/AISettingsDialogs.tsx',
    );

    for (const copy of [
      readme,
      pulsePro,
      ai,
      privacy,
      security,
      publicPrivacy,
      publicSecurity,
      pricingSpec,
      aiSettingsDialog,
    ]) {
      expect(copy).not.toMatch(/quickstart/i);
      expect(copy).not.toContain('quickstart:pulse-hosted');
      expect(copy).not.toMatch(/hosted AI/i);
      expect(copy).not.toMatch(/hosted model/i);
      expect(copy).not.toMatch(/hosted[\s\S]{0,120}no API key/i);
      expect(copy).not.toMatch(/Pulse-hosted[\s\S]{0,120}no API key/i);
      expect(copy).not.toMatch(/Pulse Account[\s\S]{0,120}no API key/i);
    }
  });

  it('keeps Relay public docs aligned with the Relay tier instead of Pro-only copy', () => {
    const security = readRepoFile('SECURITY.md');
    const publicSecurity = readRepoFile('frontend-modern/public/docs/SECURITY.md');
    const screenshots = readRepoFile('docs/SCREENSHOTS.md');

    for (const copy of [security, publicSecurity, screenshots]) {
      expect(copy).not.toContain('Relay Security (Pro)');
      expect(copy).not.toContain('Relay functionality requires a Pro or Cloud license');
      expect(copy).not.toContain('relay protocol (Pro feature)');
    }

    expect(security).toContain('Relay Security (Relay and Above)');
    expect(publicSecurity).toContain('Relay Security (Relay and Above)');
    expect(security).toContain('Relay, Pro, legacy Pro+, or Cloud license');
    expect(publicSecurity).toContain('Relay, Pro, legacy Pro+, or Cloud license');
  });
});
