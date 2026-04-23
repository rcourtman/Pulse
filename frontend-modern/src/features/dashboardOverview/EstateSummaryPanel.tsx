import { For, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { INFRASTRUCTURE_PATH } from '@/routing/resourceLinks';
import { formatRelativeTime } from '@/utils/format';
import type {
  DashboardEstateHealthTone,
  DashboardEstateSummary,
  DashboardEstateSurfaceSummary,
} from './estateSummaryModel';
import ActivityIcon from 'lucide-solid/icons/activity';
import ArrowRightIcon from 'lucide-solid/icons/arrow-right';
import ClockIcon from 'lucide-solid/icons/clock';
import ServerIcon from 'lucide-solid/icons/server';

interface EstateSummaryPanelProps {
  summary: DashboardEstateSummary;
  resourceIssueCount?: number;
  activeAlertCount?: number;
}

const toneDotClass: Record<DashboardEstateHealthTone, string> = {
  healthy: 'bg-emerald-500',
  warning: 'bg-amber-500',
  danger: 'bg-red-500',
  muted: 'bg-slate-400',
};

const toneTextClass: Record<DashboardEstateHealthTone, string> = {
  healthy: 'text-emerald-600 dark:text-emerald-400',
  warning: 'text-amber-600 dark:text-amber-400',
  danger: 'text-red-600 dark:text-red-400',
  muted: 'text-muted',
};

function formatSurfaceSummary(surfaces: DashboardEstateSurfaceSummary[]): string {
  if (surfaces.length === 0) return 'No source links yet';
  return surfaces
    .slice(0, 4)
    .map((surface) =>
      surface.label === 'Agent' && surface.count !== 1
        ? `${surface.count} agents`
        : `${surface.count} ${surface.label}`,
    )
    .join(' · ');
}

const pluralize = (count: number, singular: string, plural = `${singular}s`): string =>
  `${count} ${count === 1 ? singular : plural}`;

const reviewText = (count: number): string =>
  `${pluralize(count, 'system')} ${count === 1 ? 'needs' : 'need'} review`;

function dashboardIssueText(props: EstateSummaryPanelProps): string | null {
  const resourceIssueCount = Math.max(0, Math.trunc(props.resourceIssueCount ?? 0));
  const activeAlertCount = Math.max(0, Math.trunc(props.activeAlertCount ?? 0));

  if (resourceIssueCount > 0 && activeAlertCount > 0) {
    return `${pluralize(resourceIssueCount, 'resource issue')} and ${pluralize(activeAlertCount, 'alert')} below`;
  }
  if (resourceIssueCount > 0) return `${pluralize(resourceIssueCount, 'resource issue')} below`;
  if (activeAlertCount > 0) return `${pluralize(activeAlertCount, 'alert')} active`;
  return null;
}

function dashboardIssueSubtext(props: EstateSummaryPanelProps): string | null {
  const resourceIssueCount = Math.max(0, Math.trunc(props.resourceIssueCount ?? 0));
  const activeAlertCount = Math.max(0, Math.trunc(props.activeAlertCount ?? 0));

  if (resourceIssueCount > 0 && activeAlertCount > 0)
    return 'Resource issues and alerts listed below';
  if (resourceIssueCount > 0) return 'Resource issues listed below';
  if (activeAlertCount > 0) return 'Alerts listed below';
  return null;
}

function latestSignalSubtext(props: EstateSummaryPanelProps): string {
  const issueSubtext = dashboardIssueSubtext(props);

  if (props.summary.attentionSystems > 0) {
    if (issueSubtext) return `${reviewText(props.summary.attentionSystems)}; details below`;
    return reviewText(props.summary.attentionSystems);
  }
  if (issueSubtext) return issueSubtext;
  if (!props.summary.hasCanonicalProjection) return 'System map syncing';
  return 'No infrastructure or alert issues found';
}

export function EstateSummaryPanel(props: EstateSummaryPanelProps) {
  const latestSeenLabel = () =>
    props.summary.latestSeen
      ? formatRelativeTime(props.summary.latestSeen, { compact: true })
      : !props.summary.hasCanonicalProjection && props.summary.totalSystems > 0
        ? 'Syncing'
        : 'Waiting for signal';
  const headlineDetail = () => {
    const issueText = dashboardIssueText(props);
    return issueText ? `${props.summary.detail} · ${issueText}` : props.summary.detail;
  };

  return (
    <Card
      padding="none"
      tone="default"
      class="overflow-hidden"
      data-testid="dashboard-estate-summary"
    >
      <div class="flex flex-col gap-3 border-b border-border px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex min-w-0 items-center gap-3">
          <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-blue-50 text-blue-600 dark:bg-blue-900/40 dark:text-blue-300">
            <ServerIcon class="h-4 w-4" aria-hidden="true" />
          </div>
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <h2 class="text-sm font-semibold text-base-content">Connected infrastructure</h2>
              <span class={`h-2 w-2 rounded-full ${toneDotClass[props.summary.tone]}`} />
            </div>
            <p class="mt-0.5 text-xs text-muted">
              <span class={`font-medium ${toneTextClass[props.summary.tone]}`}>
                {props.summary.headline}
              </span>
              <span> · {headlineDetail()}</span>
            </p>
          </div>
        </div>
        <a
          href={INFRASTRUCTURE_PATH}
          class="inline-flex shrink-0 items-center gap-1.5 self-start rounded-md border border-border px-2.5 py-1.5 text-xs font-medium text-base-content hover:bg-surface-hover sm:self-auto"
        >
          View infrastructure
          <ArrowRightIcon class="h-3.5 w-3.5" aria-hidden="true" />
        </a>
      </div>

      <div class="grid grid-cols-1 gap-3 px-4 py-3 sm:grid-cols-3">
        <div>
          <p class="text-[11px] font-medium uppercase tracking-wide text-muted">Systems</p>
          <p class="mt-1 text-xl font-mono font-semibold text-base-content">
            {props.summary.totalSystems}
          </p>
          <p class="mt-0.5 text-xs text-muted">
            {props.summary.hasCanonicalProjection
              ? `${props.summary.activeSystems} active`
              : 'System map syncing'}
          </p>
        </div>

        <div>
          <p class="text-[11px] font-medium uppercase tracking-wide text-muted">Source coverage</p>
          <p class="mt-1 truncate text-sm font-medium text-base-content">
            {props.summary.hasCanonicalProjection
              ? formatSurfaceSummary(props.summary.surfaces)
              : 'Coverage syncing'}
          </p>
          <p class="mt-0.5 text-xs text-muted">
            {props.summary.hasCanonicalProjection
              ? props.summary.surfaces.length > 0
                ? 'Grouped by monitored system'
                : 'Add source coverage from Infrastructure'
              : 'Awaiting connected-infrastructure map'}
          </p>
        </div>

        <div>
          <p class="text-[11px] font-medium uppercase tracking-wide text-muted">Latest signal</p>
          <p class="mt-1 inline-flex items-center gap-1.5 text-sm font-medium text-base-content">
            <ClockIcon class="h-3.5 w-3.5 text-muted" aria-hidden="true" />
            {latestSeenLabel()}
          </p>
          <p class="mt-0.5 text-xs text-muted">{latestSignalSubtext(props)}</p>
        </div>
      </div>

      <Show when={props.summary.systems.length > 0}>
        <div class="border-t border-border px-4 py-2.5">
          <div class="flex flex-wrap items-center gap-2">
            <For each={props.summary.systems.slice(0, 5)}>
              {(system) => (
                <a
                  href={INFRASTRUCTURE_PATH}
                  class="inline-flex max-w-full items-center gap-1.5 rounded border border-border-subtle bg-base px-2 py-1 text-xs text-base-content hover:bg-surface-hover"
                  title={`${system.name} · ${system.statusLabel}`}
                >
                  <span class={`h-1.5 w-1.5 shrink-0 rounded-full ${toneDotClass[system.tone]}`} />
                  <span class="max-w-[12rem] truncate font-medium">{system.name}</span>
                  <span class="text-muted">{system.statusLabel}</span>
                </a>
              )}
            </For>
            <Show when={props.summary.systems.length > 5}>
              <a
                href={INFRASTRUCTURE_PATH}
                class="inline-flex items-center gap-1 text-xs font-medium text-blue-600 hover:underline dark:text-blue-400"
              >
                +{props.summary.systems.length - 5} more
                <ActivityIcon class="h-3 w-3" aria-hidden="true" />
              </a>
            </Show>
          </div>
        </div>
      </Show>
    </Card>
  );
}

export default EstateSummaryPanel;
