import type { Component } from 'solid-js';
import type { SecurityStatusSettingsCapabilities } from '@/types/config';
import type { SettingsTab } from './settingsRouting';

export type { SettingsTab } from './settingsRouting';

export type SettingsNavGroupId = 'infrastructure' | 'organization' | 'system' | 'security';

export interface SettingsNavItem {
  id: SettingsTab;
  label: string;
  icon: Component<{ class?: string; strokeWidth?: number }>;
  iconProps?: { strokeWidth?: number };
  saveBehavior?: 'system';
  disabled?: boolean;
  locked?: boolean;
  hideWhenUnavailable?: boolean;
  hostedOnly?: boolean;
  requiredCapability?: keyof SecurityStatusSettingsCapabilities;
  badge?: string;
  features?: string[];
  permissions?: string[];
}

export interface SettingsNavGroup {
  id: SettingsNavGroupId;
  label: string;
  items: SettingsNavItem[];
}

export interface SettingsHeaderMeta {
  title: string;
  description: string;
}

export type SettingsHeaderMetaMap = Record<SettingsTab, SettingsHeaderMeta>;
