import { apiFetchJSON } from '@/utils/apiClient';

const AVAILABILITY_HISTORY_PATH = '/api/availability-history';
const MAX_TARGETS_PER_REQUEST = 200;

export interface AvailabilityLatencySummary {
  average: number;
  min: number;
  max: number;
}

export interface AvailabilityHistorySummary {
  reachableSeconds: number;
  unreachableSeconds: number;
  indeterminateSeconds: number;
  unknownSeconds: number;
  coveragePercent: number;
  availabilityPercent?: number;
  reachableLatencyMillis?: AvailabilityLatencySummary;
}

export interface AvailabilityHistoryBucket {
  start: string;
  end: string;
  reachableSeconds: number;
  unreachableSeconds: number;
  indeterminateSeconds: number;
  unknownSeconds: number;
  latencyMillis?: AvailabilityLatencySummary;
}

export interface AvailabilityRevisionBoundary {
  revision: number;
  at: string;
}

export interface AvailabilityHistoryTargetError {
  code: 'not_found' | 'forbidden' | string;
  message: string;
}

export interface AvailabilityHistoryTarget {
  targetId: string;
  summary?: AvailabilityHistorySummary;
  buckets?: AvailabilityHistoryBucket[];
  revisionBoundaries?: AvailabilityRevisionBoundary[];
  error?: AvailabilityHistoryTargetError;
}

export interface AvailabilityHistoryResponse {
  start: string;
  end: string;
  targets: AvailabilityHistoryTarget[];
}

const uniqueTargetIds = (targetIds: readonly string[]): string[] => [
  ...new Set(targetIds.map((targetId) => targetId.trim()).filter(Boolean)),
];

export class AvailabilityHistoryAPI {
  static async batch(
    targetIds: readonly string[],
    range = '24h',
  ): Promise<AvailabilityHistoryResponse> {
    const ids = uniqueTargetIds(targetIds);
    if (ids.length === 0) {
      const now = new Date().toISOString();
      return { start: now, end: now, targets: [] };
    }

    const chunks: string[][] = [];
    for (let index = 0; index < ids.length; index += MAX_TARGETS_PER_REQUEST) {
      chunks.push(ids.slice(index, index + MAX_TARGETS_PER_REQUEST));
    }
    const responses = await Promise.all(
      chunks.map((targetIdsChunk) =>
        apiFetchJSON<AvailabilityHistoryResponse>(AVAILABILITY_HISTORY_PATH, {
          method: 'POST',
          body: JSON.stringify({ targetIds: targetIdsChunk, range }),
        }),
      ),
    );

    return {
      start: responses[0]?.start ?? new Date().toISOString(),
      end: responses[0]?.end ?? new Date().toISOString(),
      targets: responses.flatMap((response) => response.targets),
    };
  }
}
