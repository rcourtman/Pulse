import { apiFetchJSON } from '@/utils/apiClient';
import type { ResourceCertificateObservation } from '@/types/resource';

const AVAILABILITY_TARGETS_PATH = '/api/availability-targets';

export type AvailabilityProbeProtocol = 'icmp' | 'tcp' | 'udp' | 'http' | 'https';
export type AvailabilityUDPMode = 'response_required' | 'open_or_filtered';
export type AvailabilityTargetKind = 'machine' | 'service' | 'device';

export interface AvailabilityProbeStatus {
  targetId: string;
  name: string;
  targetKind?: AvailabilityTargetKind | string;
  address: string;
  protocol: AvailabilityProbeProtocol | string;
  outcome?: 'reachable' | 'unreachable' | 'indeterminate' | string;
  enabled: boolean;
  available: boolean;
  lastChecked?: string;
  lastSuccess?: string;
  latencyMillis?: number;
  consecutiveFailures?: number;
  lastError?: string;
  failureThreshold?: number;
  /**
   * Set when the latest observation was reported by a remote probe agent.
   * Absent (omitempty) when the local Pulse server ran the check.
   */
  probeAgentId?: string;
  certificate?: ResourceCertificateObservation;
}

export interface AvailabilityTarget {
  id: string;
  configRevision?: number;
  name: string;
  targetKind?: AvailabilityTargetKind;
  address: string;
  protocol: AvailabilityProbeProtocol;
  port?: number;
  path?: string;
  udpMode?: AvailabilityUDPMode;
  udpRequest?: string;
  udpExpectedResponse?: string;
  linkedResourceId?: string;
  enabled: boolean;
  pollIntervalSeconds?: number;
  timeoutMillis?: number;
  failureThreshold?: number;
  certificateMonitoringDisabled?: boolean;
  certificateExpiryWarningDays?: number;
  /**
   * Host agent that runs this check. Empty string means the local Pulse
   * server. Assigning a non-empty value requires the `external_probe`
   * entitlement. Clearing an assignment requires sending an explicit empty
   * string because the server decodes updates onto the existing record.
   */
  probeAgentId?: string;
  status?: AvailabilityProbeStatus;
}

export interface AvailabilityTestResponse {
  success: boolean;
  latencyMillis: number;
  outcome?: 'reachable' | 'unreachable' | 'indeterminate' | string;
  error?: string;
  certificate?: ResourceCertificateObservation;
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

  /**
   * Partial update. The server decodes the body onto the existing record, so
   * omitted keys keep their stored value. Send `probeAgentId: ''` explicitly to
   * clear a probe assignment and move the check back to the local Pulse server.
   */
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
