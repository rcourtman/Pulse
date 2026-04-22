import type { Accessor, Setter } from 'solid-js';

export interface NetworkSettingsPanelProps {
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
  setHasUnsavedChanges: Setter<boolean>;
}

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
