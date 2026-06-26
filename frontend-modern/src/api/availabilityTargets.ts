import { apiFetchJSON } from '@/utils/apiClient';

const AVAILABILITY_TARGETS_PATH = '/api/availability-targets';

export type AvailabilityProbeProtocol = 'icmp' | 'tcp' | 'http' | 'https';
export type AvailabilityTargetKind = 'machine' | 'service' | 'device';

export interface AvailabilityProbeStatus {
  targetId: string;
  name: string;
  targetKind?: AvailabilityTargetKind | string;
  address: string;
  protocol: AvailabilityProbeProtocol | string;
  enabled: boolean;
  available: boolean;
  lastChecked?: string;
  lastSuccess?: string;
  latencyMillis?: number;
  consecutiveFailures?: number;
  lastError?: string;
  failureThreshold?: number;
}

export interface AvailabilityTarget {
  id: string;
  name: string;
  targetKind?: AvailabilityTargetKind;
  address: string;
  protocol: AvailabilityProbeProtocol;
  port?: number;
  path?: string;
  linkedResourceId?: string;
  enabled: boolean;
  pollIntervalSeconds?: number;
  timeoutMillis?: number;
  failureThreshold?: number;
  status?: AvailabilityProbeStatus;
}

export interface AvailabilityTestResponse {
  success: boolean;
  latencyMillis: number;
  error?: string;
}

export class AvailabilityTargetsAPI {
  static async list(): Promise<AvailabilityTarget[]> {
    return apiFetchJSON(AVAILABILITY_TARGETS_PATH);
  }

  static async create(target: AvailabilityTarget): Promise<AvailabilityTarget> {
    return apiFetchJSON(AVAILABILITY_TARGETS_PATH, {
      method: 'POST',
      body: JSON.stringify(target),
    });
  }

  static async update(
    id: string,
    target: Partial<AvailabilityTarget>,
  ): Promise<AvailabilityTarget> {
    return apiFetchJSON(`${AVAILABILITY_TARGETS_PATH}/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: JSON.stringify(target),
    });
  }

  static async remove(id: string): Promise<void> {
    await apiFetchJSON(`${AVAILABILITY_TARGETS_PATH}/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
  }

  static async test(target: AvailabilityTarget): Promise<AvailabilityTestResponse> {
    return apiFetchJSON(`${AVAILABILITY_TARGETS_PATH}/test`, {
      method: 'POST',
      body: JSON.stringify(target),
    });
  }

  static async testSaved(id: string): Promise<AvailabilityTestResponse> {
    return apiFetchJSON(`${AVAILABILITY_TARGETS_PATH}/${encodeURIComponent(id)}/test`, {
      method: 'POST',
    });
  }
}
