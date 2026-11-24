import type { Accessor, Setter } from 'solid-js';
import type { VersionInfo } from '@/api/updates';

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
  versionInfo?: Accessor<VersionInfo | null>;
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
  | 'versionInfo'
  | 'webhookAllowedPrivateCIDRs'
>;

export function validatePublicURL(raw: string): string | null {
  const value = raw.trim();
  if (!value) return null;
  try {
    const parsed = new URL(value);
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      return 'Public URL must start with http:// or https://.';
    }
    return null;
  } catch {
    return 'Enter a full URL such as https://pulse.example.com.';
  }
}

export function validateOriginList(raw: string, allowWildcard = false): string | null {
  const value = raw.trim();
  if (!value) return null;
  const entries = value
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
  for (const entry of entries) {
    if (allowWildcard && entry === '*') continue;
    try {
      const parsed = new URL(entry);
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
        return `Origin "${entry}" must use http:// or https://.`;
      }
      if (
        (parsed.pathname && parsed.pathname !== '/') ||
        entry.endsWith('/') ||
        parsed.search ||
        parsed.hash
      ) {
        return `Origin "${entry}" should include only scheme, host, and optional port with no path or trailing slash.`;
      }
    } catch {
      return `Origin "${entry}" is not a valid URL origin (for example https://app.example.com).`;
    }
  }
  return null;
}

const IPV4_CIDR_RE =
  /^(?:(?:25[0-5]|2[0-4]\d|1?\d?\d)\.){3}(?:25[0-5]|2[0-4]\d|1?\d?\d)\/(?:3[0-2]|[12]?\d)$/;
const IPV6_CIDR_RE = /^[0-9a-fA-F:]+\/(?:12[0-8]|1[01]\d|[1-9]?\d)$/;

export function validatePrivateCIDRList(raw: string): string | null {
  const value = raw.trim();
  if (!value) return null;
  const entries = value
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
  for (const entry of entries) {
    const validV4 = IPV4_CIDR_RE.test(entry);
    const validV6 = entry.includes(':') && IPV6_CIDR_RE.test(entry);
    if (!validV4 && !validV6) {
      return `Invalid CIDR range "${entry}". Use format like 192.168.1.0/24 or 10.0.0.0/8.`;
    }
  }
  return null;
}
