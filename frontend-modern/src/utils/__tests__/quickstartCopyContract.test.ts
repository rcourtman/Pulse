import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../../../../');
const readRepoFile = (relativePath: string) =>
  readFileSync(resolve(repoRoot, relativePath), 'utf-8');

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

  it('keeps retired Relay guidance about existing pairings, not a current offer', () => {
    const security = readRepoFile('SECURITY.md');
    const publicSecurity = readRepoFile('frontend-modern/public/docs/SECURITY.md');
    const screenshots = readRepoFile('docs/SCREENSHOTS.md');

    for (const copy of [security, publicSecurity, screenshots]) {
      expect(copy).not.toContain('Relay Security (Pro)');
      expect(copy).not.toContain('Relay functionality requires a Pro or Cloud license');
      expect(copy).not.toContain('relay protocol (Pro feature)');
    }

    for (const copy of [security, publicSecurity]) {
      expect(copy).toContain('### Existing Mobile Pairings (Retirement)');
      expect(copy).toContain('Pulse Mobile and Relay retire on **31 March 2027**');
      expect(copy).toContain('Existing paired phones keep\nworking until then');
      expect(copy).toContain('Relay is no longer sold');
      expect(copy).toContain('receive Pro features at their current price');
      expect(copy).toContain('Paired-app access remains license-gated until retirement');
      expect(copy).toMatch(/Relay connects the app, not the\s+web UI/);
      expect(copy).not.toContain('Relay Security (Relay and Above)');
      expect(copy).not.toContain('Relay, Pro, legacy Pro+, or Cloud license');
    }
    expect(publicSecurity).toBe(security);
  });

  it('keeps FAQ and plan guidance current without offering retired Relay', () => {
    for (const file of ['FAQ.md', 'PULSE_PRO.md']) {
      const copy = readRepoFile(`docs/${file}`);
      const shippedCopy = readRepoFile(`frontend-modern/public/docs/${file}`);
      expect(shippedCopy).toBe(copy);

      const text = copy.replace(/\s+/g, ' ');
      expect(text).toContain('31 March 2027');
      expect(text).toContain('Existing paired phones keep working until then');
      expect(text).toContain('Relay is no longer sold');
      expect(text).toContain('Pro features at their current price');
      expect(text).toContain('for as long as their subscription continues');
      expect(text).toContain("Those Pro features do not end with the app's retirement");
      expect(text).toContain('Relay connects the app, not the web UI');
      expect(text).toContain('own VPN or tunnel');
      expect(text).toContain('ntfy, Gotify or Pushover');
      expect(text).not.toMatch(/Relay (?:adds|includes) secure remote (?:web )?access/i);
      expect(text).not.toContain('Remote access via Relay');
      expect(text).not.toContain('Community / Relay / Pro self-hosted plans');
      expect(copy).not.toMatch(/^\| Relay \|/m);
    }

    const plans = readRepoFile('docs/PULSE_PRO.md');
    expect(plans).toContain('Relay (legacy)');
    expect(plans).toContain('Legacy Relay payloads can still show 14-day history');
    expect(plans).toContain('Continuing Relay subscribers receive the **Pro** column');
    expect(plans).toContain('Existing Relay subscriber');
  });

  it('keeps public AI docs aligned with model-owned Patrol and Assistant reasoning', () => {
    const ai = readRepoFile('docs/AI.md');

    expect(ai).toContain('the configured LLM owns diagnosis');
    expect(ai).toContain(
      'Pulse supplies context, capabilities, safety gates, approval state, and audit trails',
    );
    expect(ai).toContain('Pulse does not convert them into Pulse-authored findings');

    expect(ai).not.toContain("learns what's normal");
    expect(ai).not.toContain('multi-layered intelligence platform');
    expect(ai).not.toContain('capacity predictions');
    expect(ai).not.toContain('Deterministic Signal Detection');
    expect(ai).not.toContain('active_alert');
    expect(ai).not.toContain('auto-recovery');
    expect(ai).not.toMatch(/understands resources before you ask/i);
  });
});
