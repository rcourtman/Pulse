import { apiErrorFromResponse, apiFetch } from '@/utils/apiClient';

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

export interface OnboardingNotReadyResponse {
  code: string;
  error?: string;
  message?: string;
  diagnostics?: OnboardingDiagnostic[];
}

export class OnboardingNotReadyError extends Error {
  readonly code: string;
  readonly status: number;
  readonly diagnostics: OnboardingDiagnostic[];

  constructor(response: OnboardingNotReadyResponse, status: number) {
    super(response.message || response.error || 'Pulse Mobile pairing is not ready yet.');
    this.name = 'OnboardingNotReadyError';
    this.code = response.code;
    this.status = status;
    this.diagnostics = response.diagnostics ?? [];
  }
}

const readJSON = async (response: Response): Promise<unknown> => {
  try {
    return await response.json();
  } catch {
    return null;
  }
};

const isOnboardingNotReadyResponse = (value: unknown): value is OnboardingNotReadyResponse => {
  if (!value || typeof value !== 'object') {
    return false;
  }
  const candidate = value as Partial<OnboardingNotReadyResponse>;
  return candidate.code === 'onboarding_not_ready';
};

export class OnboardingAPI {
  private static baseUrl = '/api/onboarding';

  static async getQRPayload(authToken?: string): Promise<OnboardingQRResponse> {
    const url = this.baseUrl + '/qr';
    const response = authToken
      ? await apiFetch(url, { headers: { 'X-API-Token': authToken } })
      : await apiFetch(url);

    if (response.ok) {
      return (await response.json()) as OnboardingQRResponse;
    }

    if (response.status === 409) {
      const body = await readJSON(response.clone());
      if (isOnboardingNotReadyResponse(body)) {
        throw new OnboardingNotReadyError(body, response.status);
      }
    }

    throw await apiErrorFromResponse(response);
  }
}
