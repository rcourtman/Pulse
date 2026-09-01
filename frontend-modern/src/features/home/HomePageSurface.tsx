import { A } from '@solidjs/router';
import { For, Show, createMemo, createSignal } from 'solid-js';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';
import ChevronUpIcon from 'lucide-solid/icons/chevron-up';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import { Button, ButtonLink } from '@/components/shared/Button';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { PageHeader } from '@/components/shared/PageHeader';
import { StatusDot } from '@/components/shared/StatusDot';
import { useResources } from '@/hooks/useResources';
import { getActiveLocale, t } from '@/i18n';
import { buildInfrastructureWorkspacePath } from '@/components/Settings/infrastructureWorkspaceModel';
import type { ResourceHealthReason, ResourceHealthVerdict } from '@/types/resource';
import {
  buildHomeAttentionTiles,
  buildHomePosture,
  buildHomeResourceGroups,
  getHomeVerdictTone,
  type HomePlatformKey,
  type HomeResourceTile,
} from './homePageModel';

const VERDICT_CLASS: Record<ResourceHealthVerdict, string> = {
  ok: 'border-emerald-300 bg-emerald-50 text-base-content dark:border-emerald-800 dark:bg-emerald-950/40',
  attention:
    'border-amber-400 bg-amber-50 text-base-content dark:border-amber-700 dark:bg-amber-950/40',
  critical: 'border-red-400 bg-red-50 text-base-content dark:border-red-800 dark:bg-red-950/40',
  stale:
    'border-dashed border-slate-400 bg-slate-50 text-slate-800 dark:border-slate-600 dark:bg-slate-900/60 dark:text-slate-200',
  off: 'border-border bg-surface text-muted',
  unknown: 'border-dashed border-slate-400 bg-surface text-muted',
};

const platformLabel = (key: HomePlatformKey): string => {
  switch (key) {
    case 'proxmox':
      return t('home.platform.proxmox');
    case 'docker':
      return t('home.platform.docker');
    case 'kubernetes':
      return t('home.platform.kubernetes');
    case 'truenas':
      return t('home.platform.truenas');
    case 'vmware':
      return t('home.platform.vmware');
    case 'standalone':
      return t('home.platform.standalone');
    default:
      return t('home.platform.other');
  }
};

const verdictLabel = (verdict: ResourceHealthVerdict): string => {
  switch (verdict) {
    case 'ok':
      return t('home.verdict.ok');
    case 'attention':
      return t('home.verdict.attention');
    case 'critical':
      return t('home.verdict.critical');
    case 'stale':
      return t('home.verdict.stale');
    case 'off':
      return t('home.verdict.off');
    default:
      return t('home.verdict.unknown');
  }
};

const reasonLabel = (
  reason: ResourceHealthReason | undefined,
  verdict: ResourceHealthVerdict,
): string => {
  if (!reason) return verdictLabel(verdict);
  const detail = reason.detail ? ` ${reason.detail}` : '';
  switch (reason.code) {
    case 'critical_alert':
      return t('home.reason.criticalAlert', { detail });
    case 'warning_alert':
      return t('home.reason.warningAlert', { detail });
    case 'availability_failed':
      return t('home.reason.availabilityFailed');
    case 'offline':
      return t('home.reason.offline');
    case 'backup_stale':
      return t('home.reason.backupStale', { detail });
    case 'telemetry_stale':
      return t('home.reason.telemetryStale', { detail });
    case 'telemetry_missing':
      return t('home.reason.telemetryMissing');
    case 'powered_off':
      return t('home.reason.poweredOff');
    case 'degraded':
      return t('home.reason.degraded');
    default:
      return verdictLabel(verdict);
  }
};

function ResourceTile(props: { tile: HomeResourceTile }) {
  const reason = () => reasonLabel(props.tile.reason, props.tile.verdict);
  return (
    <A
      href={props.tile.href}
      class={`group flex min-h-20 flex-col justify-between rounded-lg border p-3 transition-colors hover:brightness-[0.98] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 ${VERDICT_CLASS[props.tile.verdict]}`}
      aria-label={t('home.tile.ariaLabel', {
        name: props.tile.name,
        status: verdictLabel(props.tile.verdict),
        reason: reason(),
      })}
    >
      <div class="flex min-w-0 items-start gap-2">
        <StatusDot
          variant={getHomeVerdictTone(props.tile.verdict)}
          size="md"
          class="mt-1.5"
          ariaHidden
        />
        <span class="min-w-0 break-words text-sm font-semibold leading-5">{props.tile.name}</span>
      </div>
      <span class="mt-2 pl-[18px] text-xs font-medium opacity-80">{reason()}</span>
    </A>
  );
}

export default function HomePageSurface() {
  const resources = useResources();
  const [expandedGroups, setExpandedGroups] = createSignal<ReadonlySet<HomePlatformKey>>(new Set());
  const posture = createMemo(() => buildHomePosture(resources.resources()));
  const attentionTiles = createMemo(() => buildHomeAttentionTiles(resources.resources()));
  const groups = createMemo(() => buildHomeResourceGroups(resources.resources(), expandedGroups()));
  const refetch = () => resources.refetch().catch(() => undefined);
  const newestTelemetry = createMemo(() => {
    const timestamp = resources
      .resources()
      .reduce((latest, resource) => Math.max(latest, resource.lastSeen || 0), 0);
    if (!timestamp) return t('home.updated.unknown');
    return new Intl.DateTimeFormat(getActiveLocale(), {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(timestamp);
  });
  const postureText = createMemo(() => {
    const value = posture();
    if (value.total > 0 && value.ok === value.total) {
      return t('home.posture.allHealthy', { total: value.total, updated: newestTelemetry() });
    }
    return t(
      value.needsAttention === 1 ? 'home.posture.summary.singular' : 'home.posture.summary.plural',
      {
        attention: value.needsAttention,
        healthy: value.ok,
        total: value.total,
        stale: value.stale,
        unknown: value.unknown,
        updated: newestTelemetry(),
      },
    );
  });
  const toggleGroup = (key: HomePlatformKey) => {
    setExpandedGroups((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  return (
    <div class="space-y-6">
      <PageHeader
        title={t('home.title')}
        description={t('home.description')}
        descriptionVisibility="always"
        actions={
          <Button
            variant="ghost"
            size="sm"
            aria-label={t('home.refresh.ariaLabel')}
            isLoading={resources.loading()}
            onClick={() => void refetch()}
          >
            <RefreshCwIcon class="mr-2 h-4 w-4" aria-hidden="true" />
            {t('home.refresh.label')}
          </Button>
        }
      />

      <Show when={resources.loading() && resources.resources().length === 0}>
        <div
          class="flex items-center gap-2 rounded-lg border border-border bg-surface p-4 text-sm text-muted"
          role="status"
        >
          <LoadingSpinner size="sm" />
          {t('home.loading')}
        </div>
      </Show>

      <Show when={resources.error() && resources.resources().length === 0}>
        <div
          class="rounded-lg border border-red-300 bg-red-50 p-4 text-red-900 dark:border-red-800 dark:bg-red-950/40 dark:text-red-100"
          role="alert"
        >
          <p class="font-semibold">{t('home.error.title')}</p>
          <p class="mt-1 text-sm">{t('home.error.description')}</p>
          <Button class="mt-3" size="sm" onClick={() => void refetch()}>
            {t('home.error.retry')}
          </Button>
        </div>
      </Show>

      <Show when={resources.error() && resources.resources().length > 0}>
        <div
          class="flex flex-col gap-3 rounded-lg border border-amber-300 bg-amber-50 p-4 text-base-content sm:flex-row sm:items-center sm:justify-between dark:border-amber-800 dark:bg-amber-950/40"
          role="alert"
        >
          <div>
            <p class="font-semibold">{t('home.error.cached.title')}</p>
            <p class="mt-1 text-sm">{t('home.error.cached.description')}</p>
          </div>
          <Button
            class="shrink-0 self-start sm:self-auto"
            size="sm"
            isLoading={resources.loading()}
            onClick={() => void refetch()}
          >
            {t('home.error.retry')}
          </Button>
        </div>
      </Show>

      <Show when={!resources.loading() && !resources.error() && resources.resources().length === 0}>
        <section
          class="rounded-xl border border-border bg-surface p-6 text-center"
          aria-labelledby="home-empty-title"
        >
          <h2 id="home-empty-title" class="text-lg font-semibold">
            {t('home.empty.title')}
          </h2>
          <p class="mx-auto mt-2 max-w-xl text-sm text-muted">{t('home.empty.description')}</p>
          <ButtonLink class="mt-4" href={buildInfrastructureWorkspacePath()}>
            {t('home.empty.action')}
          </ButtonLink>
        </section>
      </Show>

      <Show when={resources.resources().length > 0}>
        <p
          class="border-y border-border py-3 text-sm font-medium text-base-content"
          role="status"
          aria-live="polite"
        >
          {postureText()}
        </p>

        <Show when={attentionTiles().length > 0}>
          <section class="space-y-3" aria-labelledby="home-attention-title">
            <div>
              <h2 id="home-attention-title" class="text-lg font-semibold">
                {t('home.attention.title')}
              </h2>
              <p class="text-sm text-muted">{t('home.attention.description')}</p>
            </div>
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              <For each={attentionTiles()}>{(tile) => <ResourceTile tile={tile} />}</For>
            </div>
          </section>
        </Show>

        <For each={groups()}>
          {(group) => {
            const expanded = () => expandedGroups().has(group.key);
            return (
              <section class="space-y-3" aria-labelledby={`home-group-${group.key}`}>
                <div class="flex items-end justify-between gap-3">
                  <h2 id={`home-group-${group.key}`} class="text-lg font-semibold">
                    {platformLabel(group.key)}
                  </h2>
                  <Show when={group.hiddenCount > 0 || expanded()}>
                    <button
                      type="button"
                      onClick={() => toggleGroup(group.key)}
                      class="inline-flex min-h-11 items-center gap-1 rounded-md px-2 text-sm font-medium text-blue-600 hover:bg-surface-hover focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 dark:text-blue-400"
                      aria-expanded={expanded()}
                    >
                      {expanded()
                        ? t('home.group.showLess')
                        : t('home.group.showAll', { count: group.hiddenCount })}
                      <Show
                        when={expanded()}
                        fallback={<ChevronDownIcon class="h-4 w-4" aria-hidden="true" />}
                      >
                        <ChevronUpIcon class="h-4 w-4" aria-hidden="true" />
                      </Show>
                    </button>
                  </Show>
                </div>
                <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                  <For each={group.tiles}>{(tile) => <ResourceTile tile={tile} />}</For>
                </div>
              </section>
            );
          }}
        </For>
      </Show>
    </div>
  );
}
