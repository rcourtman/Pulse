import { apiFetchJSON } from '@/utils/apiClient';

export interface LicenseStatus {
  valid: boolean;
  tier: string;
  plan_version?: string;
  email?: string;
  expires_at?: string | null;
  is_lifetime: boolean;
  days_remaining: number;
  features: string[];
  max_guests?: number;
  in_grace_period?: boolean;
  grace_period_end?: string | null;
}

export interface EntitlementLimitStatus {
  key: string;
  // 0 means unlimited
  limit: number;
  current: number;
  // "ok" | "warning" | "enforced" (string for forward-compat)
  state: string;
}

export interface EntitlementUpgradeReason {
  key: string;
  reason: string;
  action_url?: string;
}

export interface LicenseRuntimeIdentity {
  build: string;
  label: string;
  download_url?: string;
}

export interface LicenseRuntimeCapabilityBlock {
  key: string;
  reason: string;
  action_url?: string;
}

export interface EntitlementLegacyConnections {
  proxmox_nodes: number;
  docker_hosts: number;
  kubernetes_clusters: number;
}

export interface CommercialMigrationStatus {
  source?: string;
  state?: string;
  reason?: string;
  recommended_action?: string;
}

// Mirrors internal/api/subscription_entitlements.go:RuntimeCapabilitiesPayload
export interface LicenseRuntimeCapabilities {
  capabilities: string[];
  limits: EntitlementLimitStatus[];
  hosted_mode?: boolean;
  max_history_days?: number;
  runtime?: LicenseRuntimeIdentity;
  blocked_capabilities?: LicenseRuntimeCapabilityBlock[];
}

// Mirrors internal/api/subscription_entitlements.go:CommercialPosturePayload
export interface LicenseCommercialPosture {
  subscription_state: string;
  upgrade_reasons: EntitlementUpgradeReason[];
  tier: string;
  trial_expires_at?: number;
  trial_days_remaining?: number;
  trial_eligible?: boolean;
  trial_eligibility_reason?: string;
  overflow_days_remaining?: number;
  legacy_connections?: EntitlementLegacyConnections;
  has_migration_gap?: boolean;
  commercial_migration?: CommercialMigrationStatus;
}

// Mirrors internal/api/subscription_entitlements.go:EntitlementPayload
export interface LicenseCommercialEntitlements extends LicenseCommercialPosture {
  capabilities: string[];
  limits: EntitlementLimitStatus[];
  plan_version?: string;
  hosted_mode?: boolean;
  valid?: boolean;
  licensed_email?: string;
  expires_at?: string;
  is_lifetime?: boolean;
  days_remaining?: number;
  in_grace_period?: boolean;
  grace_period_end?: string;
  max_history_days?: number;
  runtime?: LicenseRuntimeIdentity;
}

export type LicenseEntitlements = LicenseCommercialEntitlements;

export interface ActivateLicenseResponse {
  success: boolean;
  message?: string;
  status?: LicenseStatus;
}

export interface ClearLicenseResponse {
  success: boolean;
  message?: string;
}

export interface LicenseFeatureStatus {
  license_status: string;
  features: Record<string, boolean>;
  upgrade_url: string;
}

function normalizeRuntimeCapabilities(payload: unknown): LicenseRuntimeCapabilities {
  const source =
    payload && typeof payload === 'object' ? (payload as Partial<LicenseRuntimeCapabilities>) : {};

  return {
    ...source,
    capabilities: Array.isArray(source.capabilities)
      ? source.capabilities.filter((value): value is string => typeof value === 'string')
      : [],
    limits: Array.isArray(source.limits)
      ? source.limits.filter(
          (value): value is EntitlementLimitStatus => Boolean(value) && typeof value === 'object',
        )
      : [],
    blocked_capabilities: Array.isArray(source.blocked_capabilities)
      ? source.blocked_capabilities.filter(
          (value): value is LicenseRuntimeCapabilityBlock =>
            Boolean(value) && typeof value === 'object',
        )
      : [],
  };
}

export class LicenseAPI {
  private static baseUrl = '/api/license';

  static async getStatus(): Promise<LicenseStatus> {
    return apiFetchJSON(`${this.baseUrl}/status`) as Promise<LicenseStatus>;
  }

  static async getRuntimeCapabilities(): Promise<LicenseRuntimeCapabilities> {
    const payload = await apiFetchJSON(`${this.baseUrl}/runtime-capabilities`);
    return normalizeRuntimeCapabilities(payload);
  }

  static async getCommercialPosture(): Promise<LicenseCommercialPosture> {
    return apiFetchJSON(`${this.baseUrl}/commercial-posture`) as Promise<LicenseCommercialPosture>;
  }

  static async getCommercialEntitlements(): Promise<LicenseCommercialEntitlements> {
    return apiFetchJSON(`${this.baseUrl}/entitlements`) as Promise<LicenseCommercialEntitlements>;
  }

  static async getEntitlements(): Promise<LicenseCommercialEntitlements> {
    return this.getCommercialEntitlements();
  }

  static async getFeatures(): Promise<LicenseFeatureStatus> {
    return apiFetchJSON(`${this.baseUrl}/features`) as Promise<LicenseFeatureStatus>;
  }

  static async activateLicense(licenseKey: string): Promise<ActivateLicenseResponse> {
    return apiFetchJSON(`${this.baseUrl}/activate`, {
      method: 'POST',
      body: JSON.stringify({ license_key: licenseKey }),
    }) as Promise<ActivateLicenseResponse>;
  }

  static async clearLicense(): Promise<ClearLicenseResponse> {
    return apiFetchJSON(`${this.baseUrl}/clear`, {
      method: 'POST',
      body: JSON.stringify({}),
    }) as Promise<ClearLicenseResponse>;
  }
}
