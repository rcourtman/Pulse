export interface ServiceHealthPresentation {
  bg: string;
  text: string;
  dot: string;
  label: string;
}

export type ServiceHealthSummaryTone = 'ok' | 'warning' | 'muted';

export interface ServiceHealthSummaryPresentation {
  tone: ServiceHealthSummaryTone;
  textClass: string;
}

const UNKNOWN_PRESENTATION: ServiceHealthPresentation = {
  bg: 'bg-surface-alt',
  text: 'text-muted',
  dot: 'bg-slate-400',
  label: 'Unknown',
};

export function getServiceHealthPresentation(
  status?: string | null,
  health?: string | null,
): ServiceHealthPresentation {
  const normalized = (health || status || '').trim().toLowerCase();

  if (normalized.includes('healthy') || normalized === 'online') {
    return {
      bg: 'bg-green-100 dark:bg-green-900',
      text: 'text-green-700 dark:text-green-400',
      dot: 'bg-green-500',
      label: 'Healthy',
    };
  }

  if (normalized.includes('degraded') || normalized.includes('warning')) {
    return {
      bg: 'bg-yellow-100 dark:bg-yellow-900',
      text: 'text-yellow-700 dark:text-yellow-400',
      dot: 'bg-yellow-500',
      label: 'Degraded',
    };
  }

  if (normalized.includes('error') || normalized === 'offline') {
    return {
      bg: 'bg-red-100 dark:bg-red-900',
      text: 'text-red-700 dark:text-red-400',
      dot: 'bg-red-500',
      label: 'Offline',
    };
  }

  if (!normalized) {
    return UNKNOWN_PRESENTATION;
  }

  return {
    ...UNKNOWN_PRESENTATION,
    label: normalized,
  };
}

export function getServiceHealthSummaryPresentation(
  status?: string | null,
  health?: string | null,
): ServiceHealthSummaryPresentation {
  const presentation = getServiceHealthPresentation(status, health);

  if (presentation.dot === 'bg-green-500') {
    return {
      tone: 'ok',
      textClass: 'text-emerald-600 dark:text-emerald-400',
    };
  }

  if (presentation.dot === 'bg-yellow-500' || presentation.dot === 'bg-red-500') {
    return {
      tone: 'warning',
      textClass: 'text-amber-600 dark:text-amber-400',
    };
  }

  return {
    tone: 'muted',
    textClass: 'text-muted',
  };
}
