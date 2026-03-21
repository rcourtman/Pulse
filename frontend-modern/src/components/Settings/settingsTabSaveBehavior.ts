import type { SettingsTab } from './settingsNavigationModel';
import { getSettingsNavItem } from './settingsNavCatalog';

export function getSettingsTabSaveBehavior(tab: SettingsTab) {
  return getSettingsNavItem(tab)?.saveBehavior;
}
