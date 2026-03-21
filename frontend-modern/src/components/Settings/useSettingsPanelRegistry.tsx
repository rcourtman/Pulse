import { createMemo } from 'solid-js';
import { createSettingsPanelRegistry } from './settingsPanelRegistry';
import {
  buildSettingsPanelRegistryContext,
  type UseSettingsPanelRegistryParams,
} from './settingsPanelRegistryContext';

export function useSettingsPanelRegistry(params: UseSettingsPanelRegistryParams) {
  return createMemo(() => createSettingsPanelRegistry(buildSettingsPanelRegistryContext(params)));
}
