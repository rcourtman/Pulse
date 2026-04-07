import { apiClient, apiFetchJSON } from '@/utils/apiClient';

export interface LicenseStatus {
  valid: boolean;
  tier: string;
  plan_version?: string;
  email?: string;
  expires_at?: string | null;
  is_lifetime: boolean;
  days_remaining: number;
  features: string[];
  max_monitored_systems?: number;
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

export class LicenseAPI {
  private static baseUrl = '/api/license';

  static async getStatus(): Promise<LicenseStatus> {
    return apiFetchJSON(`${this.baseUrl}/status`) as Promise<LicenseStatus>;
  }

  static async getRuntimeCapabilities(): Promise<LicenseRuntimeCapabilities> {
    return apiFetchJSON(
      `${this.baseUrl}/runtime-capabilities`,
    ) as Promise<LicenseRuntimeCapabilities>;
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

  static async startTrial(): Promise<Response> {
    // Return the raw Response so callers can handle status codes (409 trial_already_used, 429 rate limited, etc.)
    return apiClient.fetch(`${this.baseUrl}/trial/start`, {
      method: 'POST',
      headers: { Accept: 'application/json' },
    });
  }
}
