import type { Accessor, Setter } from 'solid-js';

export interface DiscoverySettingsFormProps {
  discoveryEnabled: Accessor<boolean>;
  discoveryMode: Accessor<'auto' | 'custom'>;
  discoverySubnetDraft: Accessor<string>;
  discoverySubnetError: Accessor<string | undefined>;
  savingDiscoverySettings: Accessor<boolean>;
  envOverrides: Accessor<Record<string, boolean>>;
  handleDiscoveryEnabledChange: (enabled: boolean) => Promise<boolean>;
  handleDiscoveryModeChange: (mode: 'auto' | 'custom') => Promise<void>;
  setDiscoveryMode: Setter<'auto' | 'custom'>;
  setDiscoverySubnetDraft: Setter<string>;
  setDiscoverySubnetError: Setter<string | undefined>;
  setLastCustomSubnet: Setter<string>;
  commitDiscoverySubnet: (value: string) => Promise<boolean>;
  parseSubnetList: (value: string) => string[];
  normalizeSubnetList: (value: string) => string;
  isValidCIDR: (value: string) => boolean;
  currentDraftSubnetValue: () => string;
  discoverySubnetInputRef?: (el: HTMLInputElement) => void;
}
