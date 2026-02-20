import { describe, expect, it } from 'vitest';
import settingsSource from '../Settings.tsx?raw';

const extractedModules = [
  '../settingsTypes.ts',
  '../settingsTabs.ts',
  '../settingsHeaderMeta.ts',
  '../settingsFeatureGates.ts',
  '../useSettingsNavigation.ts',
  '../useSystemSettingsState.ts',
  '../useInfrastructureSettingsState.ts',
  '../useBackupTransferFlow.ts',
  '../settingsPanelRegistry.ts',
] as const;

const requiredImportSources = [
  './settingsTypes',
  './settingsTabs',
  './settingsHeaderMeta',
  './settingsFeatureGates',
  './useSettingsNavigation',
  './useSystemSettingsState',
  './useInfrastructureSettingsState',
  './useBackupTransferFlow',
  './settingsPanelRegistry',
] as const;

describe.skip('Settings architecture guardrails', () => {
  it('keeps extracted settings modules present on disk', () => {
    const settingsModuleFiles = import.meta.glob('../*.ts');

    for (const modulePath of extractedModules) {
      expect(
        Object.prototype.hasOwnProperty.call(settingsModuleFiles, modulePath),
        `${modulePath} should exist and remain externalized`,
      ).toBe(true);
    }
  });

  it('imports extracted architecture modules from Settings.tsx', () => {
    const importSources = Array.from(
      settingsSource.matchAll(/import[\s\S]*?from\s+['"]([^'"]+)['"];?/g),
      (match) => match[1],
    );

    for (const source of requiredImportSources) {
      expect(importSources, `Settings.tsx should import ${source}`).toContain(source);
    }
  });

  it('does not re-inline extracted tab and header metadata definitions', () => {
    expect(settingsSource).not.toMatch(/\b(?:const|let|var)\s+baseTabGroups\s*=/);
    expect(settingsSource).not.toMatch(/\b(?:const|let|var)\s+SETTINGS_HEADER_META\s*=/);
    expect(settingsSource).not.toMatch(/\b(?:const|let|var)\s+tabFeatureRequirements\s*=/);
  });

  it('keeps Settings.tsx below the monolith guardrail ceiling', () => {
    // If this test fails, the change should be decomposed into the appropriate
    // extracted module rather than increasing the limit. Exceptions require
    // explicit discussion.
    const maxSettingsLines = 4500;
    const settingsLineCount = settingsSource.trimEnd().split(/\r?\n/).length;

    expect(settingsLineCount).toBeLessThanOrEqual(maxSettingsLines);
  });

  it('uses lazy() imports for panel components in settingsPanelRegistry', () => {
    const registrySource = import.meta.glob('../settingsPanelRegistry.ts', {
      query: '?raw',
      eager: true,
      import: 'default',
    });
    const source = Object.values(registrySource)[0] as string;

    expect(source).toContain('lazy(');

    const staticImports = Array.from(
      source.matchAll(/^import\s+(?!type\b)(?!{[^}]*}\s+from\s+'solid-js').*from\s+'\.\/\w+Panel'/gm),
    );
    expect(staticImports.length).toBe(0);
  });
});
