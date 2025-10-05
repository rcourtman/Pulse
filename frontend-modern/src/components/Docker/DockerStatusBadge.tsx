import type { JSX } from 'solid-js';

const badgeClasses = (
  status: string,
  healthyClass: string,
  warningClass: string,
  dangerClass: string,
) => {
  switch (status.toLowerCase()) {
    case 'online':
    case 'running':
    case 'healthy':
      return healthyClass;
    case 'offline':
    case 'exited':
    case 'failed':
    case 'unhealthy':
      return dangerClass;
    default:
      return warningClass;
  }
};

export const renderDockerStatusBadge = (status?: string): JSX.Element => {
  const value = (status || 'unknown').toLowerCase();
  return (
    <span
      class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold ${badgeClasses(
        value,
        'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
        'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
        'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
      )}`}
    >
      {status || 'unknown'}
    </span>
  );
};
