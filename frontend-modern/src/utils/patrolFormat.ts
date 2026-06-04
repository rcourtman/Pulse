// Pure formatting helpers for Patrol run data — no SolidJS dependencies.
import { formatIdentifierLabel } from '@/utils/textPresentation';

interface ModelInfo {
  id: string;
  name: string;
  description?: string;
  notable?: boolean;
  provider?: string;
}

interface PartialRunRecord {
  scope_resource_ids?: string[];
  effective_scope_resource_ids?: string[];
  scope_resource_types?: string[];
  type?: string;
}

export function getCanonicalScopeResourceIds(run?: PartialRunRecord | null): string[] | undefined {
  if (!run) return undefined;
  if (run.effective_scope_resource_ids !== undefined) {
    return run.effective_scope_resource_ids;
  }
  return run.scope_resource_ids;
}

export function formatDurationMs(ms?: number): string {
  if (!ms || ms <= 0) return '';
  if (ms < 1000) return `${ms}ms`;
  const seconds = Math.round(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.round(seconds / 60);
  return `${minutes}m`;
}

export function formatTriggerReason(reason?: string): string {
  switch (reason) {
    case 'scheduled':
      return 'Scheduled';
    case 'manual':
      return 'Manual';
    case 'startup':
      return 'Startup';
    case 'alert_fired':
      return 'Alert fired';
    case 'alert_cleared':
      return 'Alert cleared';
    case 'anomaly':
      return 'Anomaly';
    case 'user_action':
      return 'User action';
    case 'config_changed':
      return 'Config change';
    default:
      return formatIdentifierLabel(reason, { fallback: 'Unknown' });
  }
}

export function formatScope(run?: PartialRunRecord | null): string {
  if (!run) return '';
  const idCount = getCanonicalScopeResourceIds(run)?.length ?? 0;
  if (idCount > 0) return `Scoped to ${idCount} resource${idCount === 1 ? '' : 's'}`;
  const types = run.scope_resource_types ?? [];
  if (types.length > 0) return `Scoped to ${types.join(', ')}`;
  if (run.type === 'scoped') return 'Scoped';
  return '';
}

export function sanitizeAnalysis(text: string | undefined): string {
  if (!text) return '';
  return text
    .replace(/<｜DSML｜[^>]*>[\s\S]*?<\/｜DSML｜[^>]*>/g, '')
    .replace(/<｜DSML｜[^>]*>/g, '')
    .trim();
}

export function formatPatrolRuntimeFailureDetail(text: string | undefined): string {
  const raw = String(text || '')
    .trim()
    .replace(/\s+/g, ' ');
  if (!raw) return '';

  const lower = raw.toLowerCase();
  if (
    [
      'provider billing or quota issue',
      'selected model does not support patrol tools',
      'provider rate limited',
      'provider authentication issue',
      'provider connection issue',
      'provider not ready',
      'selected model unavailable',
      'selected model context window too small',
    ].includes(lower)
  ) {
    return raw;
  }

  if (
    lower.includes('tool_choice') ||
    lower.includes('tool calling') ||
    lower.includes('tools are not supported') ||
    (lower.includes('no endpoints found') && lower.includes('tool'))
  ) {
    return 'Provider rejected Patrol tool calls. Choose a Patrol model and endpoint with tool-call support.';
  }
  if (
    lower.includes('insufficient balance') ||
    lower.includes('402') ||
    lower.includes('payment required') ||
    lower.includes('quota') ||
    lower.includes('credit') ||
    lower.includes('max_tokens')
  ) {
    return 'Provider reported insufficient credits or token budget for the requested Patrol analysis.';
  }
  if (
    lower.includes('rate limit') ||
    lower.includes('429') ||
    lower.includes('too many requests')
  ) {
    return 'Provider rate limit reached. Wait for capacity or adjust provider limits before retrying.';
  }
  if (
    lower.includes('401') ||
    lower.includes('403') ||
    lower.includes('unauthorized') ||
    lower.includes('forbidden') ||
    lower.includes('api key')
  ) {
    return 'Provider authentication failed. Check the configured provider key and account access.';
  }
  if (
    lower.includes('failed to connect') ||
    lower.includes('connection refused') ||
    lower.includes('no such host') ||
    lower.includes('i/o timeout') ||
    lower.includes('context deadline exceeded') ||
    lower.includes('timeout') ||
    lower.includes('returned status 5')
  ) {
    return 'Provider connection failed. Check provider reachability before retrying Patrol.';
  }

  return raw
    .replace(/https?:\/\/[^\s"')]+/gi, '[redacted-url]')
    .replace(/\buser_[A-Za-z0-9_-]+\b/g, '[redacted-user]')
    .replace(/\bsk-[A-Za-z0-9_-]{8,}\b/g, '[redacted-secret]')
    .replace(/Bearer\s+[A-Za-z0-9._~+/=-]+/gi, 'Bearer [redacted-secret]')
    .replace(
      /"((?:api[_-]?key|apikey|access[_-]?token|token|authorization|x-api-key|user[_-]?id))"\s*:\s*"[^"]+"/gi,
      '"$1":"[redacted]"',
    )
    .trim();
}

export function formatPatrolRuntimeFailureSummary(input: {
  errorSummary?: string;
  errorDetail?: string;
  errorCount?: number;
}): string | undefined {
  const summary = formatPatrolRuntimeFailureDetail(input.errorSummary);
  const detail = formatPatrolRuntimeFailureDetail(input.errorDetail);
  if (summary && detail && summary !== detail) {
    return `${summary}: ${detail}`;
  }
  if (summary || detail) return summary || detail;

  const errorCount = Math.max(0, input.errorCount || 0);
  if (errorCount > 0) {
    return `${errorCount} Patrol runtime error${errorCount === 1 ? '' : 's'} recorded`;
  }
  return undefined;
}

export function groupModelsByProvider(models: ModelInfo[]): Map<string, ModelInfo[]> {
  const groups = new Map<string, ModelInfo[]>();
  for (const model of models) {
    // Prefer the server-supplied provider; fall back to the id prefix for
    // models that predate the provider field or have an opaque id (#1320).
    const provider = model.provider?.trim() || model.id.split(':')[0];
    if (!groups.has(provider)) {
      groups.set(provider, []);
    }
    groups.get(provider)!.push(model);
  }
  return groups;
}
