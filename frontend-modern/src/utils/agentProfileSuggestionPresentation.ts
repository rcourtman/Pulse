export function getAgentProfileSuggestionValueBadgeClass(value: unknown): string {
  if (typeof value === 'boolean') {
    return value
      ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
      : 'bg-surface-alt text-base-content';
  }
  return 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300';
}

export const AGENT_PROFILE_SUGGESTION_EXAMPLE_PROMPTS = [
  'Create a profile for production servers with minimal logging',
  'Profile for Docker or Podman runtimes that need container monitoring',
  'Kubernetes monitoring profile with all pods visible',
  'Development environment profile with debug logging',
] as const;

export function getAgentProfileSuggestionKeyLabel(value: string): string {
  return value
    .split('_')
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

export function formatAgentProfileSuggestionValue(value: unknown): string {
  if (value === null || value === undefined) return 'unset';
  if (typeof value === 'boolean') return value ? 'Enabled' : 'Disabled';
  if (typeof value === 'string') return value === '' ? '(empty)' : value;
  if (typeof value === 'number') return String(value);
  return JSON.stringify(value);
}

export function hasAgentProfileSuggestionValue(value: unknown): boolean {
  return value !== null && value !== undefined;
}

export function getAgentProfileSuggestionRiskHints(config: Record<string, unknown>): string[] {
  const hints: string[] = [];

  if (config.disable_auto_update === true) {
    hints.push('Auto updates are disabled. Plan manual patching for agents.');
  }
  if (config.disable_docker_update_checks === true) {
    hints.push('Docker update checks are disabled. Update visibility will be limited.');
  }
  if (config.enable_host === false) {
    hints.push('Agent monitoring is disabled. Agent metrics and command execution will stop.');
  }
  if (config.enable_docker === false) {
    hints.push('Docker monitoring is disabled. Container metrics and update tracking will stop.');
  }
  if (config.disable_ceph === true) {
    hints.push('Ceph monitoring is disabled. Cluster health checks will be skipped.');
  }

  return hints;
}

export function getAgentProfileSuggestionLoadingState() {
  return {
    text: 'Generating suggestion...',
  } as const;
}
