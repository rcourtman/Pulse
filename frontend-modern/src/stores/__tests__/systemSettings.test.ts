import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { beforeEach, describe, expect, it } from 'vitest';
import { PRIVACY_DOC_URL } from '@/utils/docsLinks';
import {
  areSystemSettingsLoaded,
  markSystemSettingsLoadedWithDefaults,
  shouldDisableLocalUpgradeMetrics,
  shouldHideDockerUpdateActions,
  shouldReduceProUpsellNoise,
  updateSystemSettingsFromResponse,
} from '@/stores/systemSettings';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const frontendRoot = path.resolve(__dirname, '..', '..', '..');
const repoRoot = path.resolve(frontendRoot, '..');

describe('systemSettings store', () => {
  beforeEach(() => {
    markSystemSettingsLoadedWithDefaults();
  });

  it('applies route and docker feature flags from API response', () => {
    updateSystemSettingsFromResponse({
      autoUpdateEnabled: false,
      disableDockerUpdateActions: true,
      reduceProUpsellNoise: true,
      disableLocalUpgradeMetrics: true,
    });

    expect(shouldHideDockerUpdateActions()).toBe(true);
    expect(shouldReduceProUpsellNoise()).toBe(true);
    expect(shouldDisableLocalUpgradeMetrics()).toBe(true);
  });

  it('resets route and docker feature flags to safe defaults', () => {
    updateSystemSettingsFromResponse({
      autoUpdateEnabled: false,
      disableDockerUpdateActions: true,
      reduceProUpsellNoise: true,
      disableLocalUpgradeMetrics: true,
    });

    markSystemSettingsLoadedWithDefaults();
    expect(shouldHideDockerUpdateActions()).toBe(false);
    expect(shouldReduceProUpsellNoise()).toBe(false);
    expect(shouldDisableLocalUpgradeMetrics()).toBe(false);
  });

  it('keeps privacy and upgrade metrics defaults safe when flags are omitted', () => {
    updateSystemSettingsFromResponse({
      autoUpdateEnabled: false,
    });

    expect(areSystemSettingsLoaded()).toBe(true);
    expect(shouldHideDockerUpdateActions()).toBe(false);
    expect(shouldReduceProUpsellNoise()).toBe(false);
    expect(shouldDisableLocalUpgradeMetrics()).toBe(false);
  });

  it('keeps the telemetry disclosure on the shipped local privacy doc', () => {
    expect(PRIVACY_DOC_URL).toBe('/docs/PRIVACY.md');
  });

  it('documents telemetry retention and field-level rationale in the privacy doc', () => {
    const privacyDoc = readFileSync(path.join(repoRoot, 'docs', 'PRIVACY.md'), 'utf8');

    expect(privacyDoc).toContain('## Usage Data');
    expect(privacyDoc).toContain('Every field is listed below with the reason it exists');
    expect(privacyDoc).toContain('rows older than **90 days** are purged automatically');
    expect(privacyDoc).toContain('uses client IP addresses transiently for abuse/rate limiting');
    expect(privacyDoc).toContain('Reset ID');
  });
});
