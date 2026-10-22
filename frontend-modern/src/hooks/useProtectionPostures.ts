import { type Accessor, createMemo } from 'solid-js';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import type {
  ProtectionPosture,
  ProtectionPosturePolicy,
  ProtectionPosturesResponse,
  ProtectionProviderState,
} from '@/types/recovery';
import { apiFetchJSON } from '@/utils/apiClient';

const PROTECTION_POSTURES_URL = '/api/recovery/postures';
export const MAX_PROTECTION_POSTURE_BATCH_SIZE = 200;
// Bound each evaluation's fan-out as well as its per-request subject count.
// Small fleets retain parallel reads; larger fleets refill only freed slots.
const PROTECTION_POSTURE_READ_CONCURRENCY = 4;
const REFRESH_MS = 30_000;
// One budget for the entire evaluation, including every batch and its body.
// Preserve the API client's existing two-minute Retry-After allowance.
export const PROTECTION_POSTURE_READ_TIMEOUT_MS = 150_000;

const EMPTY_POLICY: ProtectionPosturePolicy = {
  freshnessWindowSeconds: 0,
  verificationWindowSeconds: 0,
  requireVerification: false,
};

const EMPTY_RESPONSE: ProtectionPosturesResponse = {
  data: [],
  policy: EMPTY_POLICY,
};

export function normalizeProtectionPostureResourceIDs(
  resourceIDs: readonly string[] | null | undefined,
): string[] {
  return [
    ...new Set(
      (resourceIDs ?? []).map((resourceID) => resourceID.trim()).filter((resourceID) => resourceID),
    ),
  ].sort((left, right) => left.localeCompare(right));
}

export function buildProtectionPostureBatchURL(resourceIDs: readonly string[]): string {
  const normalized = normalizeProtectionPostureResourceIDs(resourceIDs);
  if (normalized.length > MAX_PROTECTION_POSTURE_BATCH_SIZE) {
    throw new Error(
      `Protection posture batches are limited to ${MAX_PROTECTION_POSTURE_BATCH_SIZE} resource IDs.`,
    );
  }
  const params = new URLSearchParams();
  for (const resourceID of normalized) {
    params.append('resourceId', resourceID);
  }
  params.set('limit', String(MAX_PROTECTION_POSTURE_BATCH_SIZE));
  return `${PROTECTION_POSTURES_URL}?${params.toString()}`;
}

const unreadablePosture = () => new Error('Protection posture response is unreadable.');
const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);
const isText = (value: unknown): value is string =>
  typeof value === 'string' && value.trim().length > 0;
const isOneOf = (value: unknown, choices: readonly string[]) =>
  typeof value === 'string' && choices.includes(value);
const isTime = (value: unknown) => isText(value) && Number.isFinite(Date.parse(value));
const hasOptionalTimes = (value: Record<string, unknown>, fields: readonly string[]) =>
  fields.every((field) => value[field] == null || isTime(value[field]));
// Go encodes an empty nil slice as null. Accept that wire spelling, not missing
// fields, objects or partially usable lists; normalize only after validation.
const isStringList = (value: unknown): value is string[] | null =>
  value === null || (Array.isArray(value) && value.every(isText));

function isProviderState(value: unknown): boolean {
  return (
    isRecord(value) &&
    ['provider', 'source', 'scope'].every((key) => isText(value[key])) &&
    isOneOf(value.jobState, ['success', 'warning', 'failed', 'running', 'unknown']) &&
    isOneOf(value.historyCompleteness, ['complete', 'partial', 'unavailable', 'unknown']) &&
    isOneOf(value.permissions, ['sufficient', 'partial', 'denied', 'unknown']) &&
    isStringList(value.evidenceIds) &&
    hasOptionalTimes(value, ['lastAttemptAt', 'lastSuccessAt', 'lastVerifiedAt']) &&
    (value.verificationExpected === undefined || typeof value.verificationExpected === 'boolean')
  );
}

function readPosture(value: unknown): ProtectionPosture {
  if (
    !isRecord(value) ||
    !isText(value.subjectResourceId) ||
    !isOneOf(value.state, ['protected', 'attention', 'unprotected', 'unknown']) ||
    !isOneOf(value.freshness, ['current', 'stale', 'unknown']) ||
    !isOneOf(value.verification, ['verified', 'unverified', 'stale', 'unknown']) ||
    !isOneOf(value.coverage, ['complete', 'partial', 'none', 'unknown']) ||
    !isText(value.explanation) ||
    !isTime(value.evaluatedAt) ||
    !hasOptionalTimes(value, ['lastAttemptAt', 'lastSuccessfulPointAt', 'lastVerifiedAt']) ||
    !isStringList(value.repositoryResourceIds) ||
    !isStringList(value.evidenceIds) ||
    !(
      value.providerStates === null ||
      (Array.isArray(value.providerStates) && value.providerStates.every(isProviderState))
    )
  ) {
    throw unreadablePosture();
  }
  return {
    ...(value as unknown as ProtectionPosture),
    providerStates: ((value.providerStates ?? []) as ProtectionProviderState[]).map((provider) => ({
      ...provider,
      evidenceIds: provider.evidenceIds ?? [],
    })),
    repositoryResourceIds: value.repositoryResourceIds ?? [],
    evidenceIds: value.evidenceIds ?? [],
  };
}

function readPolicy(value: unknown): ProtectionPosturePolicy {
  if (
    !isRecord(value) ||
    !['freshnessWindowSeconds', 'verificationWindowSeconds'].every(
      (key) => Number.isSafeInteger(value[key]) && (value[key] as number) > 0,
    ) ||
    typeof value.requireVerification !== 'boolean'
  ) {
    throw unreadablePosture();
  }
  return value as unknown as ProtectionPosturePolicy;
}

async function fetchProtectionPostures(
  resourceIDs: readonly string[],
  signal: AbortSignal,
): Promise<ProtectionPosturesResponse> {
  const value = await apiFetchJSON<unknown>(buildProtectionPostureBatchURL(resourceIDs), {
    signal,
  });
  if (!isRecord(value) || !Array.isArray(value.data)) throw unreadablePosture();
  const policy = readPolicy(value.policy);
  const remaining = new Set(resourceIDs);
  const data = value.data.map((raw) => {
    const posture = readPosture(raw);
    // Never let a duplicate overwrite an earlier verdict in the resource map,
    // or lend a verdict to another batch. Missing subjects remain Unknown.
    if (!remaining.delete(posture.subjectResourceId)) throw unreadablePosture();
    return posture;
  });
  return { data, policy };
}

/**
 * Fetch server-owned protection posture in bounded batches. A fleet may exceed
 * one API batch, but request count scales by 200-row pages rather than by row.
 */
export async function fetchAllProtectionPostures(
  resourceIDs: readonly string[],
  signal: AbortSignal,
): Promise<ProtectionPosturesResponse> {
  signal.throwIfAborted();
  const controller = new AbortController();
  let cancel!: () => void;
  let timer!: ReturnType<typeof setTimeout>;
  const interrupted = new Promise<never>((_resolve, reject) => {
    cancel = () => {
      reject(signal.reason);
      controller.abort();
    };
    signal.addEventListener('abort', cancel, { once: true });
    timer = setTimeout(() => {
      reject(new Error('The protection posture read timed out.'));
      controller.abort();
    }, PROTECTION_POSTURE_READ_TIMEOUT_MS);
  });
  try {
    // Abort alone need not settle a transport or partial body. Settle the
    // query's own promise too, so refresh callers and abandoned org reads do
    // not stay pending. An unreadable/denied batch cancels its siblings; no
    // partial fleet verdict or late response can become a current result.
    return await Promise.race([
      interrupted,
      readProtectionPostureBatches(resourceIDs, controller.signal),
    ]);
  } finally {
    clearTimeout(timer);
    signal.removeEventListener('abort', cancel);
    controller.abort();
  }
}

async function readProtectionPostureBatches(
  resourceIDs: readonly string[],
  signal: AbortSignal,
): Promise<ProtectionPosturesResponse> {
  const batches: string[][] = [];
  for (let offset = 0; offset < resourceIDs.length; offset += MAX_PROTECTION_POSTURE_BATCH_SIZE) {
    batches.push(resourceIDs.slice(offset, offset + MAX_PROTECTION_POSTURE_BATCH_SIZE));
  }
  const responses: ProtectionPosturesResponse[] = new Array(batches.length);
  let nextBatch = 0;
  let failed = false;
  const readNextBatch = async () => {
    while (!failed && nextBatch < batches.length) {
      // A cancelled transport can still resolve. It must not start queued
      // subjects for an abandoned owner, even when it ignored abort itself.
      signal.throwIfAborted();
      const index = nextBatch++;
      try {
        responses[index] = await fetchProtectionPostures(batches[index], signal);
      } catch (error) {
        // Stop the queue before the containing evaluation cancels siblings.
        // Never publish a partial fleet or use completion order as row order.
        failed = true;
        throw error;
      }
    }
  };
  await Promise.all(
    Array.from(
      { length: Math.min(PROTECTION_POSTURE_READ_CONCURRENCY, batches.length) },
      readNextBatch,
    ),
  );
  const policy = responses[0]?.policy ?? EMPTY_POLICY;
  if (
    responses.some(
      (response) =>
        response.policy.freshnessWindowSeconds !== policy.freshnessWindowSeconds ||
        response.policy.verificationWindowSeconds !== policy.verificationWindowSeconds ||
        response.policy.requireVerification !== policy.requireVerification,
    )
  ) {
    throw unreadablePosture();
  }
  return {
    data: responses.flatMap((response) => response.data),
    policy,
  };
}

export function useProtectionPostures(resourceIDs: Accessor<readonly string[] | null | undefined>) {
  const source = createMemo<string | null>(() => {
    const normalized = normalizeProtectionPostureResourceIDs(resourceIDs());
    return normalized.length > 0 ? JSON.stringify(normalized) : null;
  });

  const state = createNonSuspendingQuery<ProtectionPosturesResponse, string>({
    source,
    cacheKey: (key) => `protection-postures:${key}`,
    fetcher: (key, signal) => fetchAllProtectionPostures(JSON.parse(key) as string[], signal),
    initialValue: EMPTY_RESPONSE,
    pollMs: REFRESH_MS,
    retainPreviousValueOnSourceChange: false,
  });

  // The query retains fulfilled data through transient failures for continuity.
  // A protection verdict is not restore evidence: after a failed evaluation it
  // cannot remain a current "Protected" (or other) verdict under an unavailable
  // notice. Keep it withdrawn through pending retries, including cache remounts.
  const currentResponse = () => (state.error() ? EMPTY_RESPONSE : state.value());
  const postures = createMemo<ProtectionPosture[]>(() => currentResponse().data ?? []);
  const postureByResourceID = createMemo<ReadonlyMap<string, ProtectionPosture>>(
    () => new Map(postures().map((posture) => [posture.subjectResourceId, posture] as const)),
  );
  const policy = createMemo(() => currentResponse().policy ?? EMPTY_POLICY);

  return {
    response: {
      get error() {
        return state.error();
      },
      get loading() {
        return state.loading();
      },
    },
    postures,
    postureByResourceID,
    policy,
    refetch: state.refetch,
    resolvedOnce: state.resolvedOnce,
  };
}
