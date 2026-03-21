import type { SettingsTab } from './settingsTypes';
import { getSettingsNavItem } from './settingsNavCatalog';

export function getSettingsTabSaveBehavior(tab: SettingsTab) {
  return getSettingsNavItem(tab)?.saveBehavior;
}
