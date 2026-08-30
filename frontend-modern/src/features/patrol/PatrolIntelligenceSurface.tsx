import { createSignal, Show } from 'solid-js';
import ArrowRightIcon from 'lucide-solid/icons/arrow-right';
import ClipboardCheckIcon from 'lucide-solid/icons/clipboard-check';
import HistoryIcon from 'lucide-solid/icons/history';
import { ButtonLink } from '@/components/shared/Button';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { actionInboxStore } from '@/stores/actionInbox';
import { usePatrolIntelligenceState } from './usePatrolIntelligenceState';
import { PatrolIntelligenceHeader } from './PatrolIntelligenceHeader';
import { PatrolIntelligenceBanners } from './PatrolIntelligenceBanners';
import { PatrolIntelligenceWorkspace } from './PatrolIntelligenceWorkspace';
import { PatrolAttentionWorkbench } from './PatrolAttentionWorkbench';
import { PatrolObjectivesPanel } from './PatrolObjectivesPanel';
import { PatrolRecentWorkPanel } from './PatrolRecentWorkPanel';

type PatrolWorkspaceView = 'inbox' | 'protection' | 'activity';
const PATROL_WORKSPACE_VIEWS: readonly PatrolWorkspaceView[] = ['inbox', 'protection', 'activity'];

export function PatrolIntelligenceSurface() {
  const state = usePatrolIntelligenceState();
  const [activeView, setActiveView] = createSignal<PatrolWorkspaceView>('inbox');
  const [findingsOpen, setFindingsOpen] = createSignal(false);
  const workspaceTabs: Partial<Record<PatrolWorkspaceView, HTMLButtonElement>> = {};
  let findingsPanel: HTMLDetailsElement | undefined;
  const activateView = (view: PatrolWorkspaceView, focus = false) => {
    setActiveView(view);
    if (focus) queueMicrotask(() => workspaceTabs[view]?.focus());
  };
  const handleWorkspaceKeyDown = (event: KeyboardEvent, currentView: PatrolWorkspaceView) => {
    const currentIndex = PATROL_WORKSPACE_VIEWS.indexOf(currentView);
    const requestedIndex =
      event.key === 'ArrowRight'
        ? (currentIndex + 1) % PATROL_WORKSPACE_VIEWS.length
        : event.key === 'ArrowLeft'
          ? (currentIndex - 1 + PATROL_WORKSPACE_VIEWS.length) % PATROL_WORKSPACE_VIEWS.length
          : event.key === 'Home'
            ? 0
            : event.key === 'End'
              ? PATROL_WORKSPACE_VIEWS.length - 1
              : -1;
    if (requestedIndex < 0) return;
    event.preventDefault();
    activateView(PATROL_WORKSPACE_VIEWS[requestedIndex], true);
  };
  const openFindings = () => {
    activateView('activity');
    setFindingsOpen(true);
    queueMicrotask(() => {
      findingsPanel?.scrollIntoView?.({ block: 'start' });
      findingsPanel?.focus?.({ preventScroll: true });
    });
  };

  return (
    <div class="space-y-4 lg:space-y-5">
      <PatrolIntelligenceHeader state={state} />
      <PatrolIntelligenceBanners state={state} />
      <div role="tablist" aria-label="Patrol workspace" class="flex gap-1 border-b border-border">
        {PATROL_WORKSPACE_VIEWS.map((view) => (
          <button
            ref={(element) => {
              workspaceTabs[view] = element;
            }}
            type="button"
            id={`patrol-${view}-tab`}
            role="tab"
            aria-selected={activeView() === view}
            aria-controls={`patrol-${view}-panel`}
            tabindex={activeView() === view ? 0 : -1}
            class={`relative min-h-11 rounded-t-md px-4 py-2 text-sm font-medium focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 ${
              activeView() === view
                ? 'bg-surface text-base-content after:absolute after:inset-x-2 after:-bottom-px after:h-0.5 after:bg-blue-600'
                : 'text-muted hover:bg-surface-hover hover:text-base-content'
            }`}
            onClick={() => activateView(view)}
            onKeyDown={(event) => handleWorkspaceKeyDown(event, view)}
          >
            {view === 'inbox' ? 'Inbox' : view === 'protection' ? 'Protection' : 'Activity'}
          </button>
        ))}
      </div>

      <Show when={activeView() === 'inbox'}>
        <div id="patrol-inbox-panel" role="tabpanel" aria-labelledby="patrol-inbox-tab">
          <Show
            when={!state.shouldShowPatrolSetupOnly()}
            fallback={
              <div
                class={`rounded-lg border border-border bg-surface p-4 sm:p-5 ${!state.patrolEnabledLocal() ? 'pointer-events-none opacity-50' : ''}`}
              >
                <PatrolIntelligenceWorkspace state={state} />
              </div>
            }
          >
            <PatrolAttentionWorkbench
              autonomyLevel={state.autonomyLevel()}
              autonomyLocked={state.autoFixLocked()}
              pendingActionCount={actionInboxStore.pendingActionCount}
              onOpenFindings={openFindings}
            />
          </Show>
        </div>
      </Show>

      <Show when={activeView() === 'protection'}>
        <div id="patrol-protection-panel" role="tabpanel" aria-labelledby="patrol-protection-tab">
          <PatrolObjectivesPanel />
        </div>
      </Show>

      <Show when={activeView() === 'activity'}>
        <div
          id="patrol-activity-panel"
          role="tabpanel"
          aria-labelledby="patrol-activity-tab"
          class="space-y-4 lg:space-y-5"
        >
          <PatrolRecentWorkPanel />

          <section
            aria-labelledby="patrol-continuity-title"
            class="rounded-lg border border-border bg-surface"
          >
            <div class="border-b border-border px-4 py-4 sm:px-5">
              <h2 id="patrol-continuity-title" class="text-base font-semibold text-base-content">
                Review and history
              </h2>
              <p class="mt-1 max-w-3xl text-sm leading-5 text-muted">
                Audit governed operations, manage finding outcomes, or inspect Patrol history.
              </p>
            </div>
            <div class="grid divide-y divide-border sm:grid-cols-2 sm:divide-x sm:divide-y-0">
              <ButtonLink
                href="/actions"
                variant="ghost"
                size="md"
                class="group h-auto min-h-20 w-full justify-between rounded-none px-4 py-3 text-left sm:px-5"
              >
                <span class="flex min-w-0 items-start gap-3">
                  <span class="mt-0.5 inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-alt text-muted">
                    <ClipboardCheckIcon class="h-5 w-5" aria-hidden="true" />
                  </span>
                  <span class="min-w-0">
                    <span class="flex flex-wrap items-center gap-2 text-sm font-semibold text-base-content">
                      Actions and approvals
                      <Show when={actionInboxStore.pendingActionCount > 0}>
                        <MetadataBadge
                          tone="warning"
                          size="xs"
                          shape="rounded"
                          aria-label={`${actionInboxStore.pendingActionCount} ${actionInboxStore.pendingActionCount === 1 ? 'operation needs' : 'operations need'} review`}
                        >
                          {actionInboxStore.pendingActionCount} waiting
                        </MetadataBadge>
                      </Show>
                    </span>
                    <span class="mt-1 block text-xs font-normal leading-5 text-muted">
                      Review governed actions and their complete audit trail.
                    </span>
                  </span>
                </span>
                <ArrowRightIcon
                  class="h-4 w-4 shrink-0 text-muted transition-transform group-hover:translate-x-0.5"
                  aria-hidden="true"
                />
              </ButtonLink>

              <button
                type="button"
                class="group flex min-h-20 w-full items-center justify-between gap-3 px-4 py-3 text-left hover:bg-surface-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 sm:px-5"
                aria-expanded={findingsOpen()}
                aria-controls="patrol-operational-records"
                onClick={() => setFindingsOpen((open) => !open)}
              >
                <span class="flex min-w-0 items-start gap-3">
                  <span class="mt-0.5 inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-alt text-muted">
                    <HistoryIcon class="h-5 w-5" aria-hidden="true" />
                  </span>
                  <span class="min-w-0">
                    <span class="text-sm font-semibold text-base-content">
                      Finding options and history
                    </span>
                    <span class="mt-1 block text-xs font-normal leading-5 text-muted">
                      Resolve or dismiss findings, remember expected behavior, create rules, and
                      inspect check history.
                    </span>
                  </span>
                </span>
                <ArrowRightIcon
                  class={`h-4 w-4 shrink-0 text-muted transition-transform ${findingsOpen() ? 'rotate-90' : ''}`}
                  aria-hidden="true"
                />
              </button>
            </div>
          </section>

          <details
            ref={findingsPanel}
            id="patrol-operational-records"
            tabindex="-1"
            class={`rounded-lg bg-surface ${findingsOpen() ? 'border border-border' : 'border-0'}`}
            open={findingsOpen()}
            onToggle={(event) => setFindingsOpen(event.currentTarget.open)}
          >
            <summary class="sr-only">Finding options and history</summary>
            <div
              class={`space-y-4 border-t border-border p-4 sm:p-5 ${!state.patrolEnabledLocal() ? 'opacity-50 pointer-events-none' : ''}`}
            >
              <p class="text-xs leading-5 text-muted">
                Choose Finding options on an active finding to resolve it, dismiss it, remember it
                as expected, or create a suppression rule. Check history remains available here for
                the forensic trail.
              </p>
              <PatrolIntelligenceWorkspace state={state} />
            </div>
          </details>
        </div>
      </Show>
    </div>
  );
}

export default PatrolIntelligenceSurface;
