import { apiFetchJSON } from '@/utils/apiClient';

export interface OnboardingRelayDetails {
  enabled: boolean;
  url: string;
  identity_fingerprint?: string;
  identity_public_key?: string;
}

export interface OnboardingDiagnostic {
  code: string;
  severity: 'warning' | 'error';
  message: string;
  field?: string;
  expected?: string;
  received?: string;
}

export interface OnboardingQRResponse {
  schema: string;
  instance_url: string;
  instance_id?: string;
  relay: OnboardingRelayDetails;
  auth_token: string;
  deep_link: string;
  diagnostics?: OnboardingDiagnostic[];
}

export class OnboardingAPI {
  private static baseUrl = '/api/onboarding';

  static async getQRPayload(authToken?: string): Promise<OnboardingQRResponse> {
    if (!authToken) {
      return apiFetchJSON(this.baseUrl + '/qr') as Promise<OnboardingQRResponse>;
    }
    return apiFetchJSON(this.baseUrl + '/qr', {
      headers: { 'X-API-Token': authToken },
    }) as Promise<OnboardingQRResponse>;
  }
}
