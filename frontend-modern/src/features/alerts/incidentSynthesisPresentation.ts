import { t } from '@/i18n';
import type { AlertCorrelation, AlertFailureClass } from '@/types/api';

export function getAlertFailureClassLabel(failureClass?: AlertFailureClass): string {
  switch (failureClass) {
    case 'runtime':
      return t('alerts.overview.synthesis.failure.runtime');
    case 'network-path':
      return t('alerts.overview.synthesis.failure.networkPath');
    case 'application-response':
      return t('alerts.overview.synthesis.failure.applicationResponse');
    case 'certificate':
      return t('alerts.overview.synthesis.failure.certificate');
    case 'evidence-coverage':
      return t('alerts.overview.synthesis.failure.evidenceCoverage');
    default:
      return t('alerts.overview.synthesis.failure.dependency');
  }
}

export function getAlertIncidentSynthesisPresentation(correlation: AlertCorrelation) {
  const supported = correlation.inference === 'supported-cause';
  const affectedCount = correlation.affectedResourceIds?.length ?? 0;
  const observationCount = correlation.observations?.length ?? 0;
  return {
    supported,
    title: t(
      supported
        ? 'alerts.overview.synthesis.title.supported'
        : 'alerts.overview.synthesis.title.observationSet',
    ),
    badge: getAlertFailureClassLabel(correlation.failureClass),
    badgeClass: supported
      ? 'rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700 dark:bg-red-950/60 dark:text-red-300'
      : 'rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-950/60 dark:text-amber-300',
    panelClass: supported
      ? 'mb-2 rounded-lg border border-red-200 bg-red-50/70 p-3 dark:border-red-900/70 dark:bg-red-950/20'
      : 'mb-2 rounded-lg border border-amber-200 bg-amber-50/70 p-3 dark:border-amber-900/70 dark:bg-amber-950/20',
    counts: t('alerts.overview.synthesis.counts', {
      affected: affectedCount,
      observations: observationCount,
    }),
    review: t('alerts.overview.synthesis.review'),
    challenge: t(
      supported
        ? 'alerts.overview.synthesis.challenge.supported'
        : 'alerts.overview.synthesis.challenge.observationSet',
    ),
  } as const;
}
