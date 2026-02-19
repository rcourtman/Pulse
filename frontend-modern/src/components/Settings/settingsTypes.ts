import type { Component } from 'solid-js';
import type { SettingsTab } from './settingsRouting';

export type { SettingsTab } from './settingsRouting';

export type SettingsNavGroupId =
  | 'resources'
  | 'organization'
  | 'integrations'
  | 'platform';

export interface SettingsNavItem {
  id: SettingsTab;
  label: string;
  icon: Component<{ class?: string; strokeWidth?: number }>;
  iconProps?: { strokeWidth?: number };
  disabled?: boolean;
  locked?: boolean;
  hostedOnly?: boolean;
  adminOnly?: boolean;
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
