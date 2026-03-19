import type {
  ResourcePolicy,
  ResourceRedactionHint,
  ResourceRoutingScope,
  ResourceSensitivity,
} from '@/types/resource';

const asTrimmedLower = (value: unknown): string => {
  if (typeof value !== 'string') return '';
  return value.trim().toLowerCase();
};

export const normalizeResourcePolicySensitivity = (
  value?: unknown,
): ResourceSensitivity | undefined => {
  switch (asTrimmedLower(value)) {
    case 'public':
      return 'public';
    case 'internal':
      return 'internal';
    case 'sensitive':
      return 'sensitive';
    case 'restricted':
      return 'restricted';
    default:
      return undefined;
  }
};

export const normalizeResourcePolicyRoutingScope = (
  value?: unknown,
): ResourceRoutingScope | undefined => {
  switch (asTrimmedLower(value)) {
    case 'cloud-summary':
      return 'cloud-summary';
    case 'local-first':
      return 'local-first';
    case 'local-only':
      return 'local-only';
    default:
      return undefined;
  }
};

export const normalizeResourcePolicyRedactionHints = (
  value: unknown,
): ResourceRedactionHint[] | undefined => {
  if (!Array.isArray(value)) return undefined;
  const hints = value.flatMap((entry) => {
    switch (asTrimmedLower(entry)) {
      case 'hostname':
        return 'hostname';
      case 'ip-address':
        return 'ip-address';
      case 'platform-id':
        return 'platform-id';
      case 'alias':
        return 'alias';
      case 'path':
        return 'path';
      default:
        return [];
    }
  });
  return hints.length > 0 ? hints : undefined;
};

export const normalizeResourcePolicy = (policy?: {
  sensitivity?: unknown;
  routing?: {
    scope?: unknown;
    allowCloudSummary?: boolean;
    allowCloudRawSignals?: boolean;
    redact?: unknown;
  };
}): ResourcePolicy | undefined => {
  const sensitivity = normalizeResourcePolicySensitivity(policy?.sensitivity);
  const scope = normalizeResourcePolicyRoutingScope(policy?.routing?.scope);
  if (!sensitivity || !scope) return undefined;
  return {
    sensitivity,
    routing: {
      scope,
      allowCloudSummary: policy?.routing?.allowCloudSummary === true,
      allowCloudRawSignals: policy?.routing?.allowCloudRawSignals === true,
      redact: normalizeResourcePolicyRedactionHints(policy?.routing?.redact),
    },
  };
};
