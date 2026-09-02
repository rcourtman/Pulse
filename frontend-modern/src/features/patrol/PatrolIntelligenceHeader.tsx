import { createMemo, Show } from 'solid-js';
import { A } from '@solidjs/router';
import PlayIcon from 'lucide-solid/icons/play';
import SettingsIcon from 'lucide-solid/icons/settings';
import DownloadIcon from 'lucide-solid/icons/download';
import { PulsePatrolLogo } from '@/components/Brand/PulsePatrolLogo';
import { PageHeader } from '@/components/shared/PageHeader';
import { TogglePrimitive } from '@/components/shared/Toggle';
import { CountdownTimer } from '@/components/patrol';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { Button } from '@/components/shared/Button';
import { UpgradeButtonLink } from '@/components/shared/UpgradeLink';
import type { PatrolAutonomyLevel } from '@/api/patrol';
import { settingsTabPath } from '@/components/Settings/settingsNavigationModel';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { formatRelativeTime } from '@/utils/format';
import { getPatrolPageHeaderMeta } from '@/utils/patrolPagePresentation';
import { getPatrolTriggerStatusSummary } from '@/utils/patrolRunPresentation';
import { getPatrolSetupAction } from '@/utils/patrolRuntimeActions';
import { getPatrolRecencyPresentation } from '@/utils/patrolSummaryPresentation';
import { PATROL_CONTROL_ANCHOR, PATROL_OPERATIONS_LOOP_ANCHOR } from '@/routing/resourceLinks';
import type { PatrolConfigurationFailureInput } from './patrolInvestigationContextModel';
import { getPatrolAutonomyAvailabilityPresentation } from './patrolAutonomyAvailability';
import { PATROL_AUTONOMY_POLICY_PRESENTATION } from './patrolControlPresentation';
import { PATROL_AUTONOMY_EXPERIENCE } from './patrolHomePresentation';
import {
  resolvePatrolBlockedActionCause,
  type PatrolIntelligenceState,
} from './usePatrolIntelligenceState';
import { PatrolAutopilotAcknowledgementDialog } from './PatrolAutopilotAcknowledgementDialog';

export { PATROL_AUTONOMY_POLICY_PRESENTATION } from './patrolControlPresentation';

const isNonEmptyConfigurationDetail = (value?: string | null): value is string =>
  Boolean(value?.trim());

export function getPatrolConfigurationFailureInlineDetails(
  failure: PatrolConfigurationFailureInput,
): string[] {
  const readiness = failure.readiness ?? null;
  const codeAndCause = [failure.code, readiness?.cause || failure.blockedCause]
    .filter(isNonEmptyConfigurationDetail)
    .join(' · ');

  return [
    codeAndCause || undefined,
    readiness?.summary ? `Setup: ${readiness.summary}` : undefined,
    readiness?.provider ? `Provider: ${readiness.provider}` : undefined,
    readiness?.model ? `Model: ${readiness.model}` : undefined,
  ].filter(isNonEmptyConfigurationDetail);
}

export function getPatrolAutopilotExpiry(expiresAt?: string | null): Date | null {
  if (!expiresAt?.trim()) return null;
  const expiry = new Date(expiresAt);
  if (!Number.isFinite(expiry.getTime()) || expiry.getUTCFullYear() <= 1) return null;
  return expiry;
}

export function PatrolIntelligenceHeader(props: { state: PatrolIntelligenceState }) {
  const state = props.state;
  const autopilotExpiry = createMemo(() =>
    getPatrolAutopilotExpiry(state.autopilotStatus()?.expiresAt),
  );
  const headerMeta = createMemo(() =>
    getPatrolPageHeaderMeta({
      autonomyLevel: state.autonomyLevel(),
      autonomyLocked: state.autoFixLocked(),
    }),
  );
  const recency = createMemo(() =>
    getPatrolRecencyPresentation({
      runs: state.patrolRunHistory.value() ?? [],
      lastPatrolAt: state.patrolStatus()?.last_patrol_at,
      lastActivityAt: state.patrolStatus()?.last_activity_at,
    }),
  );
  const triggerStatusSummary = createMemo(() =>
    getPatrolTriggerStatusSummary(state.patrolStatus()?.trigger_status, {
      manualRunAvailable: state.canTriggerPatrol(),
      manualRunBlockedReason: state.triggerPatrolDisabledReason(),
    }),
  );
  const providerSetupAction = () =>
    getPatrolSetupAction(
      resolvePatrolBlockedActionCause(state.blockedCause(), state.patrolReadiness()?.cause),
    );
  const runControlBusy = createMemo(
    () =>
      state.isTriggeringPatrol() || state.manualRunRequested() || state.patrolStream.isStreaming(),
  );
  const runBlockedByProviderSetup = createMemo(
    () =>
      !runControlBusy() &&
      state.patrolEnabledLocal() &&
      state.runtimeState() === 'active' &&
      state.patrolReadiness()?.status === 'not_ready',
  );
  const runButtonDisabled = createMemo(() => runControlBusy() || !state.canTriggerPatrol());
  const runButtonLabel = createMemo(() =>
    state.isTriggeringPatrol()
      ? 'Starting…'
      : state.manualRunRequested() || state.patrolStream.isStreaming()
        ? 'Running…'
        : 'Check now',
  );
  const renderRunControl = (className: string) => (
    <Show
      when={!runBlockedByProviderSetup()}
      fallback={
        <A
          href={providerSetupAction().href}
          aria-label={`${providerSetupAction().label}: ${state.triggerPatrolDisabledReason() || 'Patrol setup needs attention'}`}
          title={state.triggerPatrolDisabledReason() || providerSetupAction().label}
          class={className}
        >
          <SettingsIcon class="w-4 h-4" />
          Fix setup
        </A>
      }
    >
      <button
        onClick={() => state.handleRunPatrol()}
        disabled={runButtonDisabled()}
        title={state.triggerPatrolDisabledReason()}
        class={className}
      >
        <PlayIcon class={`w-4 h-4 ${runControlBusy() ? 'animate-pulse' : ''}`} />
        {runButtonLabel()}
      </button>
    </Show>
  );
  const effectiveAutonomyLevel = createMemo<PatrolAutonomyLevel>(() =>
    state.autoFixLocked() ? 'monitor' : state.autonomyLevel(),
  );
  const selectedAutonomyPolicy = createMemo(
    () => PATROL_AUTONOMY_POLICY_PRESENTATION[effectiveAutonomyLevel()],
  );
  const selectedAutonomyExperience = createMemo(
    () => PATROL_AUTONOMY_EXPERIENCE[effectiveAutonomyLevel()],
  );
  const upgradePromptsHidden = createMemo(() => presentationPolicyHidesUpgradePrompts());
  const canChooseAutonomyLevel = createMemo(() => !state.autoFixLocked());
  const autonomyAvailability = createMemo(() =>
    getPatrolAutonomyAvailabilityPresentation({
      autoFixLocked: state.autoFixLocked(),
      upgradePromptsHidden: upgradePromptsHidden(),
      commercialSurfacesHidden: true,
      runtimeCapabilityBlock: state.autoFixCapabilityBlock(),
      runtime: state.licenseRuntimeIdentity(),
      planUpgradeDestination: getUpgradeActionDestination('ai_autofix'),
    }),
  );
  const shouldShowAutonomyOptions = createMemo(
    () => canChooseAutonomyLevel() || autonomyAvailability().kind === 'runtime_locked',
  );
  const showAutonomyUpgradeAction = createMemo(
    () =>
      !presentationPolicyHidesUpgradePrompts() &&
      state.autoFixLocked() &&
      autonomyAvailability().kind === 'runtime_locked',
  );
  const shouldShowAutonomyActionColumn = createMemo(() => shouldShowAutonomyOptions());
  const showAutonomyAvailabilityPrompt = createMemo(
    () =>
      state.autoFixLocked() &&
      autonomyAvailability().kind === 'runtime_locked' &&
      Boolean(autonomyAvailability().actionLabel),
  );
  const showAutonomyAvailabilityNote = createMemo(
    () =>
      state.autoFixLocked() &&
      autonomyAvailability().kind === 'runtime_locked' &&
      !autonomyAvailability().actionLabel,
  );
  const autonomyLevelOptions = createMemo<FilterOption<PatrolAutonomyLevel>[]>(() =>
    (['monitor', 'approval', 'assisted', 'full'] as const).map((level) => {
      const presentation = PATROL_AUTONOMY_POLICY_PRESENTATION[level];
      const lockedPaidMode = state.autoFixLocked() && level !== 'monitor';
      return {
        value: level,
        label: presentation.label,
        compactLabel: presentation.compactLabel,
        title: presentation.detail,
        disabled: lockedPaidMode,
      };
    }),
  );
  const renderAutonomyPolicyControl = (options: {
    ariaLabel: string;
    layoutClass: string;
    controlClass: string;
    variant?: 'segmented' | 'prominent';
  }) => (
    <>
      <div class={options.layoutClass}>
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-xs font-semibold uppercase tracking-wider text-muted">
              Patrol mode
            </span>
            <span class="rounded border border-border-subtle bg-surface px-2 py-0.5 text-xs font-medium text-base-content">
              {selectedAutonomyPolicy().label}
            </span>
          </div>
          <p class="mt-1 text-sm leading-5 text-muted">{selectedAutonomyPolicy().detail}</p>
        </div>

        <Show when={shouldShowAutonomyActionColumn()}>
          <div class="flex w-full flex-col gap-2 lg:w-auto">
            <Show when={shouldShowAutonomyOptions()}>
              <FilterButtonGroup
                ariaLabel={options.ariaLabel}
                class={options.controlClass}
                disabled={!state.patrolEnabledLocal()}
                options={autonomyLevelOptions()}
                value={effectiveAutonomyLevel()}
                onChange={(level) => state.handleAutonomyChange(level)}
                variant={options.variant ?? 'segmented'}
              />
            </Show>
          </div>
        </Show>
      </div>

      <Show when={showAutonomyAvailabilityNote()}>
        <p class="mt-2 text-xs leading-5 text-muted">{autonomyAvailability().body}</p>
      </Show>
      <Show when={showAutonomyAvailabilityPrompt()}>
        <div class="mt-3 flex flex-col gap-3 rounded-md border border-blue-200 bg-blue-50 px-3 py-3 dark:border-blue-800 dark:bg-blue-950/40 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <p class="text-sm font-semibold text-blue-950 dark:text-blue-100">
              {autonomyAvailability().title}
            </p>
            <p class="mt-1 text-xs leading-5 text-blue-800 dark:text-blue-200">
              {autonomyAvailability().body}
            </p>
          </div>
          <Show when={showAutonomyUpgradeAction() && autonomyAvailability().destination?.href}>
            <UpgradeButtonLink
              destination={autonomyAvailability().destination!}
              size="sm"
              mobileFullWidth={false}
              class="shrink-0"
            >
              <DownloadIcon class="h-4 w-4" />
              {autonomyAvailability().actionLabel}
            </UpgradeButtonLink>
          </Show>
        </div>
      </Show>
      <Show when={state.isUpdatingAutonomy()}>
        <div role="status" class="sr-only">
          Saving Patrol mode
        </div>
      </Show>
      <Show when={state.requestedAutonomyLevel() !== state.autonomyLevel()}>
        <div
          role="status"
          class="mt-3 rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-200"
        >
          Requested {PATROL_AUTONOMY_POLICY_PRESENTATION[state.requestedAutonomyLevel()].label}.
          Effective mode is {selectedAutonomyPolicy().label}. Server status:{' '}
          {state.autopilotStatus()?.code.replace(/_/g, ' ') || 'unavailable'}.
        </div>
      </Show>
      <Show when={state.autopilotStatus()?.active && state.autopilotStatus()?.acknowledgementId}>
        <div class="mt-3 flex flex-col gap-2 rounded-md border border-border bg-surface px-3 py-3 text-xs sm:flex-row sm:items-center sm:justify-between">
          <div>
            <span class="font-semibold">
              Autopilot acknowledgement v{state.autopilotStatus()?.acknowledgementVersion}
            </span>
            <span class="ml-2 text-muted">
              Active for this identity
              <Show when={autopilotExpiry()}>
                {(expiry) => <> until {expiry().toLocaleString()}</>}
              </Show>
              .
            </span>
          </div>
          <Button
            size="sm"
            variant="dangerOutline"
            disabled={state.isUpdatingAutonomy()}
            onClick={() => void state.revokeAutopilot()}
          >
            Revoke Autopilot
          </Button>
        </div>
      </Show>
    </>
  );

  return (
    <div class="space-y-4">
      <PatrolAutopilotAcknowledgementDialog state={state} />
      <PageHeader
        id="patrol-title"
        description={headerMeta().description}
        title={
          <span class="inline-flex items-center gap-3" title={headerMeta().titleTooltip}>
            <PulsePatrolLogo class="w-6 h-6 text-base-content" decorative />
            <span>{headerMeta().title}</span>
          </span>
        }
        class="relative z-[200] mb-3"
      />

      <section id={PATROL_CONTROL_ANCHOR} class="border-y border-border">
        <div class="flex flex-col gap-3 px-1 py-2 sm:px-2 lg:flex-row lg:items-center lg:justify-between">
          <div class="flex min-w-0 items-start gap-3">
            <div class="pt-0.5">
              <TogglePrimitive
                checked={state.patrolEnabledLocal()}
                disabled={state.isTogglingPatrol()}
                onToggle={state.handleTogglePatrol}
                size="sm"
                ariaLabel="Toggle Patrol"
              />
            </div>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="text-sm font-semibold text-base-content">
                  {state.patrolEnabledLocal() ? 'Watching your infrastructure' : 'Patrol is off'}
                </h2>
                <span class="rounded-full border border-blue-200 bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-800 dark:border-blue-800 dark:bg-blue-950/50 dark:text-blue-200">
                  {selectedAutonomyExperience().label}
                </span>
              </div>
              <Show when={!state.patrolEnabledLocal()}>
                <p class="mt-1 max-w-3xl text-sm leading-5 text-muted">
                  Turn Patrol on when you want it to watch your estate in the background.
                </p>
              </Show>
              <Show when={!state.shouldShowPatrolSetupOnly() && triggerStatusSummary()}>
                <p class="mt-1 text-xs leading-5 text-muted">{triggerStatusSummary()}</p>
              </Show>
              <Show when={!shouldShowAutonomyActionColumn()}>
                <div class="mt-1 flex flex-wrap items-baseline gap-x-2 text-xs leading-5 text-muted">
                  <span class="font-semibold text-base-content">Patrol mode</span>
                  <span>{selectedAutonomyPolicy().detail}</span>
                </div>
              </Show>
              <Show when={!state.shouldShowPatrolSetupOnly() && recency().timestamp}>
                <div class="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs leading-5 text-muted">
                  <span>
                    {recency().label}:{' '}
                    {formatRelativeTime(recency().timestamp, {
                      compact: true,
                      emptyText: 'Never',
                    })}
                  </span>
                  <Show when={recency().resourcesCheckedLabel}>
                    <span aria-hidden="true">·</span>
                    <span>{recency().resourcesCheckedLabel}</span>
                  </Show>
                  <Show when={state.patrolStatus()?.next_patrol_at}>
                    <span aria-hidden="true">·</span>
                    <CountdownTimer
                      targetDate={state.patrolStatus()!.next_patrol_at!}
                      prefix="Next check "
                      class="font-variant-numeric tabular-nums"
                    />
                  </Show>
                </div>
              </Show>
            </div>
          </div>

          <Show when={!state.shouldShowPatrolSetupOnly()}>
            <div class="grid w-full shrink-0 grid-cols-2 gap-2 lg:flex lg:w-auto lg:flex-wrap lg:items-center">
              {renderRunControl(
                'flex min-h-11 items-center justify-center gap-2 rounded-md border border-border bg-surface px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:bg-surface-alt disabled:text-muted sm:min-h-0',
              )}
              <Show when={!runBlockedByProviderSetup()}>
                <A
                  href={settingsTabPath('system-ai-patrol')}
                  aria-label="Open Patrol settings"
                  title="Open Patrol settings"
                  class="flex min-h-11 items-center justify-center gap-2 rounded-md border border-transparent px-3 py-1.5 text-sm font-medium text-muted transition-colors hover:bg-surface-hover hover:text-base-content sm:min-h-0"
                >
                  <SettingsIcon class="h-4 w-4" />
                  Settings
                </A>
              </Show>
            </div>
          </Show>
        </div>

        <span id={PATROL_OPERATIONS_LOOP_ANCHOR} class="sr-only" aria-hidden="true" />
        <Show when={shouldShowAutonomyActionColumn()}>
          <details class="border-t border-border-subtle px-1 py-1 sm:px-2">
            <summary class="min-h-11 cursor-pointer text-xs font-medium text-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 sm:min-h-0">
              Mode and automation
            </summary>
            <div class="pt-3">
              {renderAutonomyPolicyControl({
                ariaLabel: 'Patrol mode',
                layoutClass: 'flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between',
                controlClass: 'w-full lg:w-[34rem]',
              })}
            </div>
          </details>
        </Show>
      </section>
    </div>
  );
}
