import type { Accessor, Setter } from 'solid-js';

export interface NetworkSettingsPanelProps {
  discoveryEnabled: Accessor<boolean>;
  discoveryMode: Accessor<'auto' | 'custom'>;
  discoverySubnetDraft: Accessor<string>;
  discoverySubnetError: Accessor<string | undefined>;
  savingDiscoverySettings: Accessor<boolean>;
  envOverrides: Accessor<Record<string, boolean>>;
  allowedOrigins: Accessor<string>;
  setAllowedOrigins: Setter<string>;
  allowEmbedding: Accessor<boolean>;
  setAllowEmbedding: Setter<boolean>;
  allowedEmbedOrigins: Accessor<string>;
  setAllowedEmbedOrigins: Setter<string>;
  webhookAllowedPrivateCIDRs: Accessor<string>;
  setWebhookAllowedPrivateCIDRs: Setter<string>;
  publicURL: Accessor<string>;
  setPublicURL: Setter<string>;
  handleDiscoveryEnabledChange: (enabled: boolean) => Promise<boolean>;
  handleDiscoveryModeChange: (mode: 'auto' | 'custom') => Promise<void>;
  setDiscoveryMode: Setter<'auto' | 'custom'>;
  setDiscoverySubnetDraft: Setter<string>;
  setDiscoverySubnetError: Setter<string | undefined>;
  setLastCustomSubnet: Setter<string>;
  commitDiscoverySubnet: (value: string) => Promise<boolean>;
  setHasUnsavedChanges: Setter<boolean>;
  parseSubnetList: (value: string) => string[];
  normalizeSubnetList: (value: string) => string;
  isValidCIDR: (value: string) => boolean;
  currentDraftSubnetValue: () => string;
  discoverySubnetInputRef?: (el: HTMLInputElement) => void;
}

export type NetworkDiscoverySectionProps = Pick<
  NetworkSettingsPanelProps,
  | 'commitDiscoverySubnet'
  | 'currentDraftSubnetValue'
  | 'discoveryEnabled'
  | 'discoveryMode'
  | 'discoverySubnetDraft'
  | 'discoverySubnetError'
  | 'discoverySubnetInputRef'
  | 'envOverrides'
  | 'handleDiscoveryEnabledChange'
  | 'handleDiscoveryModeChange'
  | 'isValidCIDR'
  | 'normalizeSubnetList'
  | 'parseSubnetList'
  | 'savingDiscoverySettings'
  | 'setDiscoveryMode'
  | 'setDiscoverySubnetDraft'
  | 'setDiscoverySubnetError'
  | 'setLastCustomSubnet'
>;

export type NetworkBoundarySettingsSectionProps = Pick<
  NetworkSettingsPanelProps,
  | 'allowEmbedding'
  | 'allowedEmbedOrigins'
  | 'allowedOrigins'
  | 'envOverrides'
  | 'publicURL'
  | 'setAllowEmbedding'
  | 'setAllowedEmbedOrigins'
  | 'setAllowedOrigins'
  | 'setHasUnsavedChanges'
  | 'setPublicURL'
  | 'setWebhookAllowedPrivateCIDRs'
  | 'webhookAllowedPrivateCIDRs'
>;
