import { createMemo, createSignal, createUniqueId, type Setter } from 'solid-js';
import { GuestMetadataAPI } from '@/api/guestMetadata';
import { notificationStore } from '@/stores/notifications';
import { dispatchResourceMetadataChanged } from '@/utils/resourceMetadataEvents';
import {
  buildWorkloadWebLinkSourceRows,
  filterWorkloadWebLinkRows,
  formatWorkloadWebLinkCount,
  getWorkloadWebLinkChanges,
  getWorkloadWebLinksNoun,
  keepStableWorkloadWebLinkRows,
  validateWorkloadWebLinkChanges,
  type WorkloadWebLinkChange,
  type WorkloadWebLinkDrafts,
  type WorkloadWebLinkFilter,
  type WorkloadWebLinkRow,
  type WorkloadWebLinksSource,
} from './workloadWebLinksModel';

export interface WorkloadWebLinksStateProps {
  source: WorkloadWebLinksSource;
  /** Owned by the table-header trigger so closing the panel keeps typed links. */
  drafts: () => WorkloadWebLinkDrafts;
  setDrafts: Setter<WorkloadWebLinkDrafts>;
  onClose: () => void;
}

/**
 * Bulk web-link panel state. The panel only mounts while open, so its live
 * inventory reads add no per-tick work to a table whose editor is closed.
 */
export function useWorkloadWebLinksState(props: WorkloadWebLinksStateProps) {
  const formId = `workload-web-links-${createUniqueId()}`;
  const noun = getWorkloadWebLinksNoun(props.source);
  const [filter, setFilter] = createSignal<WorkloadWebLinkFilter>('all');
  const [errors, setErrors] = createSignal<Record<string, string>>({});
  const [saving, setSaving] = createSignal(false);
  // Resource-backed tables only learn a saved link on their next refetch, so
  // hold what this panel just saved rather than flash the stale value back.
  const [justSaved, setJustSaved] = createSignal<Record<string, string>>({});

  const rows = createMemo<WorkloadWebLinkRow[]>((previous) => {
    const saved = justSaved();
    const next = buildWorkloadWebLinkSourceRows(props.source).map((row) =>
      row.metadataId in saved ? { ...row, savedUrl: saved[row.metadataId] } : row,
    );
    return keepStableWorkloadWebLinkRows(previous, next);
  }, []);
  const visibleRows = createMemo(() => filterWorkloadWebLinkRows(rows(), filter()));
  const linkedCount = createMemo(() => rows().filter((row) => row.savedUrl).length);
  const changes = createMemo(() => getWorkloadWebLinkChanges(rows(), props.drafts()));

  const setDraft = (row: WorkloadWebLinkRow, value: string) => {
    // Typing a link back to its saved value drops the draft, so the trigger's
    // unsaved marker only reflects real edits.
    props.setDrafts(({ [row.metadataId]: _previous, ...rest }) =>
      value === row.savedUrl ? rest : { ...rest, [row.metadataId]: value },
    );
    if (errors()[row.metadataId]) {
      setErrors(({ [row.metadataId]: _cleared, ...rest }) => rest);
    }
  };

  const discardChanges = () => {
    props.setDrafts({});
    setErrors({});
  };

  const focusFirstError = (failed: Record<string, string>) => {
    const firstId = rows().find((row) => failed[row.metadataId])?.metadataId;
    if (!firstId) return;
    queueMicrotask(() =>
      Array.from(document.querySelectorAll<HTMLInputElement>(`#${formId} input`))
        .find((input) => input.dataset.metadataId === firstId)
        ?.focus(),
    );
  };

  const save = async () => {
    const pending = changes();
    if (pending.length === 0 || saving()) return;

    const invalid = validateWorkloadWebLinkChanges(pending);
    if (Object.keys(invalid).length > 0) {
      setErrors(invalid);
      setFilter('all');
      focusFirstError(invalid);
      return;
    }

    setSaving(true);
    const failed: Record<string, string> = {};
    const saved: WorkloadWebLinkChange[] = [];
    for (const change of pending) {
      try {
        await GuestMetadataAPI.updateMetadata(change.metadataId, { customUrl: change.url });
        saved.push(change);
      } catch (error) {
        failed[change.metadataId] =
          error instanceof Error && error.message ? error.message : 'Could not save this link.';
      }
    }

    setJustSaved((current) => ({
      ...current,
      ...Object.fromEntries(saved.map((change) => [change.metadataId, change.url])),
    }));
    props.setDrafts((current) => {
      const next = { ...current };
      for (const change of saved) delete next[change.metadataId];
      return next;
    });
    // One burst after the writes: listeners that refetch whole snapshots
    // coalesce onto the same in-flight request instead of one per link.
    for (const change of saved) {
      dispatchResourceMetadataChanged({
        metadataKind: 'guest',
        metadataId: change.metadataId,
        customUrl: change.url,
      });
    }
    setSaving(false);
    setErrors(failed);

    if (Object.keys(failed).length === 0) {
      notificationStore.success(`Saved ${formatWorkloadWebLinkCount(saved.length)}.`);
      props.onClose();
      return;
    }
    focusFirstError(failed);
  };

  return {
    formId,
    noun,
    filter,
    setFilter,
    drafts: props.drafts,
    setDraft,
    discardChanges,
    errors,
    saving,
    rows,
    visibleRows,
    linkedCount,
    changes,
    save,
    close: props.onClose,
  };
}

export type WorkloadWebLinksState = ReturnType<typeof useWorkloadWebLinksState>;
