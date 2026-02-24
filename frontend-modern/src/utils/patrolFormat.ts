// Pure formatting helpers for Patrol run data — no SolidJS dependencies.

interface ModelInfo {
  id: string;
  name: string;
  description: string;
  notable: boolean;
}

interface PartialRunRecord {
  scope_resource_ids?: string[];
  scope_resource_types?: string[];
  type?: string;
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
      return reason ? reason.replace(/_/g, ' ') : 'Unknown';
  }
}

export function formatScope(run?: PartialRunRecord | null): string {
  if (!run) return '';
  const idCount = run.scope_resource_ids?.length ?? 0;
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

export function groupModelsByProvider(models: ModelInfo[]): Map<string, ModelInfo[]> {
  const groups = new Map<string, ModelInfo[]>();
  for (const model of models) {
    const [provider] = model.id.split(':');
    if (!groups.has(provider)) {
      groups.set(provider, []);
    }
    groups.get(provider)!.push(model);
  }
  return groups;
}
