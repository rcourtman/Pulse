export function getAlertHistoryResourceTypeBadgeClass(resourceType?: string | null): string {
  const normalized = (resourceType ?? '').trim().toLowerCase();

  if (normalized === 'vm' || normalized === 'node') {
    return 'text-xs px-1 py-0.5 rounded bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300';
  }

  // 'system-container' is the unified-model name the v6 alert engine stamps
  // for LXC guests (guest_snapshot.go resourceType()).
  if (
    normalized === 'container' ||
    normalized === 'ct' ||
    normalized === 'lxc' ||
    normalized === 'system-container'
  ) {
    return 'text-xs px-1 py-0.5 rounded bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300';
  }

  if (normalized === 'storage') {
    return 'text-xs px-1 py-0.5 rounded bg-orange-100 dark:bg-orange-900 text-orange-700 dark:text-orange-300';
  }

  return 'text-xs px-1 py-0.5 rounded bg-surface-hover text-base-content';
}
