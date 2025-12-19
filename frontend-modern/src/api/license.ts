import { apiFetchJSON } from '@/utils/apiClient';

export interface LicenseStatus {
  valid: boolean;
  tier: string;
  email?: string;
  expires_at?: string | null;
  is_lifetime: boolean;
  days_remaining: number;
  features: string[];
  max_nodes?: number;
  max_guests?: number;
  in_grace_period?: boolean;
  grace_period_end?: string | null;
}

export interface ActivateLicenseResponse {
  success: boolean;
  message?: string;
  status?: LicenseStatus;
}

export interface ClearLicenseResponse {
  success: boolean;
  message?: string;
}

export class LicenseAPI {
  private static baseUrl = '/api/license';

  static async getStatus(): Promise<LicenseStatus> {
    return apiFetchJSON(`${this.baseUrl}/status`) as Promise<LicenseStatus>;
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
