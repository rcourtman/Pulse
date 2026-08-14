import { For, Show, createMemo, createSignal, onCleanup, onMount, type Component } from 'solid-js';
import PlusIcon from 'lucide-solid/icons/plus';
import PencilIcon from 'lucide-solid/icons/pencil';
import PauseIcon from 'lucide-solid/icons/pause';
import PlayIcon from 'lucide-solid/icons/play';
import TrashIcon from 'lucide-solid/icons/trash-2';
import XIcon from 'lucide-solid/icons/x';
import { Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { ResourcePicker, type SelectedResource } from '@/components/Settings/ResourcePicker';
import {
  createPatrolObjective,
  deletePatrolObjective,
  getPatrolObjectives,
  updatePatrolObjective,
  type PatrolObjective,
  type PatrolObjectiveCoverageState,
} from '@/api/patrol';
import { useResources } from '@/hooks/useResources';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';
import { showError, showSuccess } from '@/utils/toast';
import { getPatrolObjectiveProtectionSummary } from './patrolHomePresentation';

const coveragePresentation = (
  state: PatrolObjectiveCoverageState,
): { label: string; tone: 'success' | 'warning' | 'neutral' } => {
  switch (state) {
    case 'covered':
      return { label: 'Watching in background', tone: 'success' };
    case 'degraded':
      return { label: 'Monitoring needs attention', tone: 'warning' };
    default:
      return { label: 'Not monitored yet', tone: 'neutral' };
  }
};

const formatObjectiveError = (error: unknown): string =>
  error instanceof Error ? error.message : 'The objective could not be saved.';

export const PatrolObjectivesPanel: Component = () => {
  let dialogReturnFocus: HTMLElement | null = null;
  const { resources } = useResources();
  const [objectives, setObjectives] = createSignal<PatrolObjective[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [loadError, setLoadError] = createSignal('');
  const [dialogOpen, setDialogOpen] = createSignal(false);
  const [editing, setEditing] = createSignal<PatrolObjective | null>(null);
  const [brief, setBrief] = createSignal('');
  const [context, setContext] = createSignal('');
  const [selectedResources, setSelectedResources] = createSignal<SelectedResource[]>([]);
  const [scopeOpen, setScopeOpen] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [mutatingId, setMutatingId] = createSignal('');

  const resourceById = createMemo(
    () => new Map(resources().map((resource) => [resource.id, resource])),
  );
  const protectionSummary = createMemo(() => getPatrolObjectiveProtectionSummary(objectives()));

  const loadObjectives = async (quiet = false) => {
    if (!quiet) setLoading(true);
    try {
      setObjectives(await getPatrolObjectives());
      setLoadError('');
    } catch (error) {
      setLoadError(formatObjectiveError(error));
    } finally {
      if (!quiet) setLoading(false);
    }
  };

  onMount(() => {
    void loadObjectives();
    const refresh = () => {
      if (document.visibilityState === 'visible' && !dialogOpen()) void loadObjectives(true);
    };
    const timer = window.setInterval(refresh, 15_000);
    document.addEventListener('visibilitychange', refresh);
    onCleanup(() => {
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', refresh);
    });
  });

  const resetForm = () => {
    setEditing(null);
    setBrief('');
    setContext('');
    setSelectedResources([]);
    setScopeOpen(false);
    setSaving(false);
  };

  const restoreDialogFocus = () => {
    const target = dialogReturnFocus;
    dialogReturnFocus = null;
    queueMicrotask(() => {
      if (target && document.contains(target)) target.focus();
    });
  };

  const closeDialog = () => {
    if (saving()) return;
    setDialogOpen(false);
    resetForm();
    restoreDialogFocus();
  };

  const openCreate = (trigger: HTMLElement) => {
    dialogReturnFocus = trigger;
    resetForm();
    setDialogOpen(true);
  };

  const openEdit = (objective: PatrolObjective, trigger: HTMLElement) => {
    dialogReturnFocus = trigger;
    setEditing(objective);
    setBrief(objective.brief);
    setContext(objective.optional_context ?? '');
    const selected = objective.scope.resource_ids.map((id) => {
      const resource = resourceById().get(id);
      return {
        id,
        type: resource?.type ?? 'agent',
        name: resource ? getPreferredInfrastructureDisplayName(resource) : id,
      } satisfies SelectedResource;
    });
    setSelectedResources(selected);
    setScopeOpen(selected.length > 0);
    setDialogOpen(true);
  };

  const saveObjective = async () => {
    const normalizedBrief = brief().trim();
    if (!normalizedBrief) return;
    setSaving(true);
    try {
      const current = editing();
      const payload = {
        brief: normalizedBrief,
        optional_context: context().trim(),
        resource_ids: selectedResources().map((resource) => resource.id),
      };
      if (current) {
        await updatePatrolObjective(current.id, { revision: current.revision, ...payload });
        showSuccess(
          'Patrol objective updated',
          'Patrol is rebuilding truthful background coverage.',
        );
      } else {
        await createPatrolObjective(payload);
        showSuccess('Patrol objective added', 'Patrol is setting up background monitoring.');
      }
      setDialogOpen(false);
      resetForm();
      restoreDialogFocus();
      await loadObjectives(true);
    } catch (error) {
      showError('Could not save Patrol objective', formatObjectiveError(error));
      setSaving(false);
    }
  };

  const setObjectiveStatus = async (objective: PatrolObjective, status: 'active' | 'paused') => {
    setMutatingId(objective.id);
    try {
      await updatePatrolObjective(objective.id, { revision: objective.revision, status });
      await loadObjectives(true);
    } catch (error) {
      showError('Could not update Patrol objective', formatObjectiveError(error));
    } finally {
      setMutatingId('');
    }
  };

  const removeObjective = async (objective: PatrolObjective) => {
    if (!window.confirm(`Delete “${objective.brief}”? Patrol will stop watching this outcome.`))
      return;
    setMutatingId(objective.id);
    try {
      await deletePatrolObjective(objective.id, objective.revision);
      setObjectives((current) => current.filter((item) => item.id !== objective.id));
      showSuccess('Patrol objective deleted');
    } catch (error) {
      showError('Could not delete Patrol objective', formatObjectiveError(error));
    } finally {
      setMutatingId('');
    }
  };

  return (
    <section
      class="rounded-lg border border-border bg-surface"
      aria-labelledby="patrol-objectives-title"
    >
      <div class="flex flex-col gap-3 border-b border-border px-4 py-4 sm:flex-row sm:items-start sm:justify-between sm:px-5">
        <div>
          <h2 id="patrol-objectives-title" class="text-base font-semibold text-base-content">
            What Patrol is looking after
          </h2>
          <p class="mt-1 max-w-3xl text-sm text-muted">
            Describe the outcome. Patrol chooses a cheap local signal, wakes the model only when
            evidence changes, and handles any fix according to your Patrol mode.
          </p>
        </div>
        <Button variant="primary" size="sm" onClick={(event) => openCreate(event.currentTarget)}>
          <PlusIcon class="mr-2 h-4 w-4" />
          Add objective
        </Button>
      </div>

      <div class="p-4 sm:p-5">
        <Show
          when={!loading()}
          fallback={<p class="py-6 text-center text-sm text-muted">Loading objectives…</p>}
        >
          <Show
            when={!loadError()}
            fallback={
              <div class="rounded-lg border border-red-300 bg-red-50 p-4 text-sm text-red-900 dark:border-red-900 dark:bg-red-950/30 dark:text-red-100">
                <p>Patrol objectives could not be loaded.</p>
                <Button class="mt-3" size="sm" onClick={() => void loadObjectives()}>
                  Try again
                </Button>
              </div>
            }
          >
            <div
              class={`mb-4 rounded-lg border px-4 py-3 ${
                protectionSummary().tone === 'success'
                  ? 'border-emerald-200 bg-emerald-50/70 dark:border-emerald-900 dark:bg-emerald-950/20'
                  : protectionSummary().tone === 'warning'
                    ? 'border-amber-200 bg-amber-50/70 dark:border-amber-900 dark:bg-amber-950/20'
                    : 'border-border-subtle bg-surface-alt/40'
              }`}
              aria-live="polite"
            >
              <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
                <p class="text-sm font-semibold text-base-content">
                  {protectionSummary().headline}
                </p>
                <Show when={protectionSummary().paused > 0}>
                  <MetadataBadge tone="neutral" size="xs" shape="rounded">
                    {protectionSummary().paused} paused
                  </MetadataBadge>
                </Show>
              </div>
              <p class="mt-1 text-xs leading-5 text-muted">{protectionSummary().detail}</p>
            </div>
            <Show
              when={objectives().length > 0}
              fallback={
                <div class="rounded-lg border border-dashed border-border px-4 py-8 text-center">
                  <p class="text-sm font-medium text-base-content">No retained outcomes yet</p>
                  <p class="mx-auto mt-1 max-w-xl text-sm text-muted">
                    Try “Keep my cameras available” or “Keep disk use below 85%”. You set the
                    outcome; Patrol works out how to observe it safely.
                  </p>
                </div>
              }
            >
              <div class="divide-y divide-border rounded-lg border border-border">
                <For each={objectives()}>
                  {(objective) => {
                    const presentation = () => coveragePresentation(objective.coverage.state);
                    const scopeLabel = () =>
                      objective.scope.resource_ids.length === 0
                        ? 'Entire estate'
                        : `${objective.scope.resource_ids.length} selected ${objective.scope.resource_ids.length === 1 ? 'resource' : 'resources'}`;
                    return (
                      <article class="flex flex-col gap-3 p-4 lg:flex-row lg:items-start lg:justify-between">
                        <div class="min-w-0 flex-1">
                          <div class="flex flex-wrap items-center gap-2">
                            <h3 class="break-words text-sm font-semibold text-base-content">
                              {objective.brief}
                            </h3>
                            <Show when={objective.status === 'paused'}>
                              <MetadataBadge tone="neutral" size="xs" shape="rounded">
                                Paused
                              </MetadataBadge>
                            </Show>
                            <MetadataBadge tone={presentation().tone} size="xs" shape="rounded">
                              {presentation().label}
                            </MetadataBadge>
                          </div>
                          <p class="mt-1 text-sm text-muted">{objective.coverage.summary}</p>
                          <p class="mt-2 text-xs text-muted">{scopeLabel()}</p>
                        </div>
                        <div class="flex flex-wrap items-center gap-2">
                          <Button
                            variant="ghost"
                            size="sm"
                            disabled={mutatingId() === objective.id}
                            onClick={() =>
                              void setObjectiveStatus(
                                objective,
                                objective.status === 'paused' ? 'active' : 'paused',
                              )
                            }
                          >
                            {objective.status === 'paused' ? (
                              <PlayIcon class="mr-2 h-4 w-4" />
                            ) : (
                              <PauseIcon class="mr-2 h-4 w-4" />
                            )}
                            {objective.status === 'paused' ? 'Resume' : 'Pause'}
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={(event) => openEdit(objective, event.currentTarget)}
                          >
                            <PencilIcon class="mr-2 h-4 w-4" />
                            Edit
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            disabled={mutatingId() === objective.id}
                            onClick={() => void removeObjective(objective)}
                          >
                            <TrashIcon class="mr-2 h-4 w-4" />
                            Delete
                          </Button>
                        </div>
                      </article>
                    );
                  }}
                </For>
              </div>
            </Show>
          </Show>
        </Show>
      </div>

      <Dialog
        isOpen={dialogOpen()}
        onClose={closeDialog}
        closeOnBackdrop={!saving()}
        ariaLabelledBy="patrol-objective-dialog-title"
        ariaDescribedBy="patrol-objective-dialog-description"
        panelClass="max-w-3xl"
      >
        <div class="flex max-h-[min(92vh,900px)] flex-col">
          <header class="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
            <div>
              <h2
                id="patrol-objective-dialog-title"
                class="text-lg font-semibold text-base-content"
              >
                {editing() ? 'Edit Patrol objective' : 'Add a Patrol objective'}
              </h2>
              <p id="patrol-objective-dialog-description" class="mt-1 text-sm text-muted">
                State the outcome in your own words. Avoid writing commands or implementation steps.
              </p>
            </div>
            <Button
              variant="ghost"
              size="icon"
              aria-label="Close objective dialog"
              onClick={closeDialog}
            >
              <XIcon class="h-5 w-5" />
            </Button>
          </header>
          <div class="space-y-5 overflow-y-auto px-5 py-4">
            <label class="block">
              <span class="text-sm font-medium text-base-content">
                What should Patrol keep true?
              </span>
              <textarea
                autofocus
                rows="3"
                maxlength="2048"
                value={brief()}
                onInput={(event) => setBrief(event.currentTarget.value)}
                placeholder="Keep Jellyfin playback from buffering for users"
                class="mt-2 w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/30"
              />
            </label>
            <label class="block">
              <span class="text-sm font-medium text-base-content">Useful context (optional)</span>
              <textarea
                rows="2"
                maxlength="4096"
                value={context()}
                onInput={(event) => setContext(event.currentTarget.value)}
                placeholder="For example: prefer local event evidence and avoid interrupting active playback"
                class="mt-2 w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/30"
              />
            </label>
            <div class="rounded-lg border border-border">
              <button
                type="button"
                class="flex w-full items-center justify-between gap-3 px-4 py-3 text-left text-sm font-medium text-base-content"
                aria-expanded={scopeOpen()}
                onClick={() => setScopeOpen((value) => !value)}
              >
                <span>Limit to specific resources</span>
                <span class="text-xs font-normal text-muted">
                  {selectedResources().length === 0
                    ? 'Entire estate'
                    : `${selectedResources().length} selected`}
                </span>
              </button>
              <Show when={scopeOpen()}>
                <div class="border-t border-border p-4">
                  <ResourcePicker
                    maxSelection={64}
                    selected={selectedResources}
                    onSelectionChange={setSelectedResources}
                  />
                </div>
              </Show>
            </div>
          </div>
          <footer class="flex flex-col-reverse gap-2 border-t border-border px-5 py-4 sm:flex-row sm:justify-end">
            <Button onClick={closeDialog} disabled={saving()}>
              Cancel
            </Button>
            <Button
              variant="primary"
              disabled={!brief().trim()}
              isLoading={saving()}
              onClick={() => void saveObjective()}
            >
              {editing() ? 'Save and rebuild monitoring' : 'Add objective'}
            </Button>
          </footer>
        </div>
      </Dialog>
    </section>
  );
};

export default PatrolObjectivesPanel;
