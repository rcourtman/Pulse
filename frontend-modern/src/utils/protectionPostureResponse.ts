import type {
  ProtectionPosture,
  ProtectionPosturePolicy,
  ProtectionPosturesResponse,
  ProtectionProviderState,
} from '@/types/recovery';

export class ProtectionPostureFormatError extends Error {
  constructor() {
    // The response may contain private source identifiers or proxy content.
    super('The protection posture response format is invalid.');
    this.name = 'ProtectionPostureFormatError';
  }
}

type Check = (value: unknown) => boolean;
const text: Check = (value) => typeof value === 'string' && value.trim().length > 0;
const timestamp: Check = (value) => text(value) && Number.isFinite(Date.parse(value as string));
const optionalTime: Check = (value) => value == null || timestamp(value);
const boolean: Check = (value) => typeof value === 'boolean';
const positiveSeconds: Check = (value) =>
  typeof value === 'number' && Number.isSafeInteger(value) && value > 0;
const choice =
  (...values: string[]): Check =>
  (value) =>
    typeof value === 'string' && values.includes(value);
const idList: Check = (value) => value === null || (Array.isArray(value) && value.every(text));

function record(value: unknown, shape: Readonly<Record<string, Check>>): Record<string, unknown> {
  if (
    value === null ||
    typeof value !== 'object' ||
    Array.isArray(value) ||
    !Object.entries(shape).every(([key, check]) => check((value as Record<string, unknown>)[key]))
  ) {
    throw new ProtectionPostureFormatError();
  }
  return value as Record<string, unknown>;
}

function list(value: unknown): unknown[] {
  // Empty Go slices may be encoded as null, but a missing collection is not
  // an observed empty result. Never drop an invalid row and accept the rest.
  if (value === null) return [];
  if (!Array.isArray(value)) throw new ProtectionPostureFormatError();
  return value;
}

const providerShape = {
  provider: text,
  source: text,
  scope: text,
  jobState: choice('success', 'warning', 'failed', 'running', 'unknown'),
  historyCompleteness: choice('complete', 'partial', 'unavailable', 'unknown'),
  permissions: choice('sufficient', 'partial', 'denied', 'unknown'),
  lastAttemptAt: optionalTime,
  lastSuccessAt: optionalTime,
  lastVerifiedAt: optionalTime,
  evidenceIds: idList,
  verificationExpected: (value: unknown) => value === undefined || boolean(value),
};
const postureShape = {
  subjectResourceId: text,
  state: choice('protected', 'attention', 'unprotected', 'unknown'),
  freshness: choice('current', 'stale', 'unknown'),
  verification: choice('verified', 'unverified', 'stale', 'unknown'),
  coverage: choice('complete', 'partial', 'none', 'unknown'),
  lastAttemptAt: optionalTime,
  lastSuccessfulPointAt: optionalTime,
  lastVerifiedAt: optionalTime,
  repositoryResourceIds: idList,
  evidenceIds: idList,
  explanation: text,
  evaluatedAt: timestamp,
};
const policyShape = {
  freshnessWindowSeconds: positiveSeconds,
  verificationWindowSeconds: positiveSeconds,
  requireVerification: boolean,
};

/** Validate the whole read before publishing any batch's assessments or policy. */
export function parseProtectionPostureBatches(
  payloads: readonly unknown[],
): ProtectionPosturesResponse {
  const data: ProtectionPosture[] = [];
  const subjects = new Set<string>();
  let policy: ProtectionPosturePolicy | undefined;
  for (const payload of payloads) {
    const batch = record(payload, {});
    const batchPolicy = record(batch.policy, policyShape) as unknown as ProtectionPosturePolicy;
    if (
      policy &&
      (policy.freshnessWindowSeconds !== batchPolicy.freshnessWindowSeconds ||
        policy.verificationWindowSeconds !== batchPolicy.verificationWindowSeconds ||
        policy.requireVerification !== batchPolicy.requireVerification)
    ) {
      throw new ProtectionPostureFormatError();
    }
    policy = batchPolicy;
    for (const value of list(batch.data)) {
      const row = record(value, postureShape);
      const subject = row.subjectResourceId as string;
      // A Map would otherwise silently choose the last conflicting assessment.
      if (subjects.has(subject)) throw new ProtectionPostureFormatError();
      subjects.add(subject);
      const providerStates = list(row.providerStates).map((provider) => {
        const checked = record(provider, providerShape);
        return {
          ...checked,
          evidenceIds: checked.evidenceIds ?? [],
        } as unknown as ProtectionProviderState;
      });
      data.push({
        ...row,
        providerStates,
        repositoryResourceIds: row.repositoryResourceIds ?? [],
        evidenceIds: row.evidenceIds ?? [],
      } as unknown as ProtectionPosture);
    }
  }
  if (!policy) throw new ProtectionPostureFormatError();
  return { data, policy };
}
