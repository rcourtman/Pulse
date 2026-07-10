import { createMemo, Show } from 'solid-js';
import { A } from '@solidjs/router';
import PlayIcon from 'lucide-solid/icons/play';
import SettingsIcon from 'lucide-solid/icons/settings';
import DownloadIcon from 'lucide-solid/icons/download';
import CreditCardIcon from 'lucide-solid/icons/credit-card';
import { PulsePatrolLogo } from '@/components/Brand/PulsePatrolLogo';
import { PageHeader } from '@/components/shared/PageHeader';
import { TogglePrimitive } from '@/components/shared/Toggle';
import { CountdownTimer } from '@/components/patrol';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { UpgradeButtonLink } from '@/components/shared/UpgradeLink';
import type { PatrolAutonomyLevel } from '@/api/patrol';
import { settingsTabPath } from '@/components/Settings/settingsNavigationModel';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import {
  presentationPolicyHidesCommercialSurfaces,
  presentationPolicyHidesUpgradePrompts,
} from '@/stores/sessionPresentationPolicy';
import { formatRelativeTime } from '@/utils/format';
import { getPatrolPageHeaderMeta } from '@/utils/patrolPagePresentation';
import { getPatrolTriggerStatusSummary } from '@/utils/patrolRunPresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';
import { getPatrolSetupAction } from '@/utils/patrolRuntimeActions';
import { getPatrolRecencyPresentation } from '@/utils/patrolSummaryPresentation';
import { PATROL_CONTROL_ANCHOR, PATROL_OPERATIONS_LOOP_ANCHOR } from '@/routing/resourceLinks';
import type { PatrolConfigurationFailureInput } from './patrolInvestigationContextModel';
import { getPatrolAutonomyAvailabilityPresentation } from './patrolAutonomyAvailability';
import { PATROL_AUTONOMY_POLICY_PRESENTATION } from './patrolControlPresentation';
import type { PatrolIntelligenceState } from './usePatrolIntelligenceState';

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

export function PatrolIntelligenceHeader(props: { state: PatrolIntelligenceState }) {
  const state = props.state;
  const headerMeta = createMemo(() =>
    getPatrolPageHeaderMeta({
      autonomyLevel: state.autonomyLevel(),
      autonomyLocked: state.autoFixLocked(),
    }),
  );
  const runtimePresentation = createMemo(() =>
    getPatrolRuntimePresentation(state.runtimeState(), state.blockedReason()),
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
  const providerSetupAction = () => getPatrolSetupAction(state.patrolReadiness()?.cause);
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
        : 'Run Patrol',
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
  const upgradePromptsHidden = createMemo(() => presentationPolicyHidesUpgradePrompts());
  const commercialSurfacesHidden = createMemo(() => presentationPolicyHidesCommercialSurfaces());
  const canChooseAutonomyLevel = createMemo(() => !state.autoFixLocked());
  const autonomyAvailability = createMemo(() =>
    getPatrolAutonomyAvailabilityPresentation({
      autoFixLocked: state.autoFixLocked(),
      upgradePromptsHidden: upgradePromptsHidden(),
      commercialSurfacesHidden: commercialSurfacesHidden(),
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
  const showAutonomyPlanBillingAction = createMemo(
    () =>
      state.autoFixLocked() &&
      autonomyAvailability().kind === 'plan_locked' &&
      Boolean(autonomyAvailability().actionLabel && autonomyAvailability().destination?.href),
  );
  const shouldShowAutonomyActionColumn = createMemo(
    () => shouldShowAutonomyOptions() || showAutonomyPlanBillingAction(),
  );
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
      const showProBadge = lockedPaidMode && !commercialSurfacesHidden();
      return {
        value: level,
        label: presentation.label,
        compactLabel: presentation.compactLabel,
        ariaLabel: showProBadge ? `${presentation.label} Pro` : undefined,
        title: lockedPaidMode
          ? `${presentation.detail} Available with Pulse Pro.`
          : presentation.detail,
        disabled: lockedPaidMode,
        visualLabel: showProBadge ? (
          <>
            <span>{presentation.label}</span>
            <span class="rounded border border-blue-200 bg-blue-50 px-1 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-blue-700 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-300">
              Pro
            </span>
          </>
        ) : undefined,
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
            <Show when={showAutonomyPlanBillingAction()}>
              <UpgradeButtonLink
                destination={autonomyAvailability().destination!}
                size="sm"
                mobileFullWidth={false}
                class="self-start lg:self-end"
              >
                <CreditCardIcon class="h-4 w-4" />
                {autonomyAvailability().actionLabel}
              </UpgradeButtonLink>
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
    </>
  );

  return (
    <div class="space-y-4">
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
        actions={
          <div class="hidden sm:flex flex-wrap items-center justify-end gap-3">
            <Show when={!state.shouldShowPatrolSetupOnly()}>
              <>
                <Show when={recency().timestamp}>
                  <div class="flex items-center gap-3 text-xs text-muted">
                    <span>
                      {recency().label}:{' '}
                      {formatRelativeTime(recency().timestamp, {
                        compact: true,
                        emptyText: 'Never',
                      })}
                      <Show when={recency().resourcesCheckedLabel}>
                        {' '}
                        <span class="text-muted">— {recency().resourcesCheckedLabel}</span>
                      </Show>
                    </span>
                    <Show when={state.patrolStatus()?.next_patrol_at}>
                      <span class="text-muted">|</span>
                      <CountdownTimer
                        targetDate={state.patrolStatus()!.next_patrol_at!}
                        prefix="Next run: "
                        class="font-variant-numeric tabular-nums font-medium text-blue-600 dark:text-blue-400"
                      />
                    </Show>
                  </div>
                </Show>

                {renderRunControl(
                  'flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:bg-surface-alt disabled:text-muted rounded-md transition-colors',
                )}
              </>
            </Show>
          </div>
        }
      />

      <div class="flex flex-wrap items-center gap-3">
        <div class="flex items-center gap-2">
          <TogglePrimitive
            checked={state.patrolEnabledLocal()}
            disabled={state.isTogglingPatrol()}
            onToggle={state.handleTogglePatrol}
            size="sm"
            ariaLabel="Toggle Patrol"
          />
          <span class="text-sm font-medium text-base-content">{runtimePresentation().label}</span>
        </div>

        <Show when={!state.shouldShowPatrolSetupOnly() && triggerStatusSummary()}>
          <span class="max-w-full text-xs leading-5 text-muted">{triggerStatusSummary()}</span>
        </Show>

        <Show when={!state.shouldShowPatrolSetupOnly()}>
          <div class="flex flex-wrap items-center gap-2 sm:ml-auto">
            {renderRunControl(
              'sm:hidden flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:bg-surface-alt disabled:text-muted rounded-md transition-colors',
            )}

            <Show when={!runBlockedByProviderSetup()}>
              <A
                href={settingsTabPath('system-ai-patrol')}
                aria-label="Open Patrol settings"
                title="Open Patrol settings"
                class="flex items-center gap-2 rounded-md border border-border px-3 py-1.5 text-sm font-medium text-base-content shadow-sm transition-colors hover:bg-surface-alt"
              >
                <SettingsIcon class="w-4 h-4" />
                <span class="sr-only sm:not-sr-only">Settings</span>
              </A>
            </Show>
          </div>
        </Show>
      </div>

      <div
        id={PATROL_CONTROL_ANCHOR}
        class="rounded-md border border-border-subtle bg-surface-alt/50 px-3 py-3"
      >
        <span id={PATROL_OPERATIONS_LOOP_ANCHOR} class="sr-only" aria-hidden="true" />
        {renderAutonomyPolicyControl({
          ariaLabel: 'Patrol mode',
          layoutClass: 'flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between',
          controlClass: 'w-full lg:w-[34rem]',
        })}
      </div>
    </div>
  );
}
