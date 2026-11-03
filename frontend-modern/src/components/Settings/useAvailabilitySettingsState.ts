import { batch, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { unwrap } from 'solid-js/store';
import {
  AvailabilityTargetsAPI,
  type AvailabilityTarget,
  type AvailabilityTargetKind,
} from '@/api/availabilityTargets';
import { apiErrorStatus } from '@/api/responseUtils';
import { getOrgID } from '@/utils/apiClient';

export type AvailabilityDialogState =
  | { mode: 'add'; initialTargetKind?: AvailabilityTargetKind }
  | { mode: 'edit'; target: AvailabilityTarget }
  | null;

export const AVAILABILITY_READ_ERROR =
  'Saved availability checks could not be loaded. Retry only reads the current configuration; it does not change checks.';
export const AVAILABILITY_ACCESS_ERROR =
  'Access to saved availability checks could not be confirmed. Retry to check access again.';

export function useAvailabilitySettingsState(props: {
  org: string;
  isCurrentReader: () => boolean;
}) {
  const [targets, setTargets] = createSignal<AvailabilityTarget[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [loaded, setLoaded] = createSignal(false);
  const [loadError, setLoadError] = createSignal<string | null>(null);
  const [dialog, setDialog] = createSignal<AvailabilityDialogState>(null);
  const [pendingActionId, setPendingActionId] = createSignal<string | null>(null);
  const [deleteConfirmingId, setDeleteConfirmingId] = createSignal<string | null>(null);
  const controller = new AbortController();
  let readController: AbortController | undefined;
  let disposed = false;
  const isCurrent = () =>
    !disposed && props.isCurrentReader() && (getOrgID() || 'default') === props.org;
  const requestOptions = (signal = controller.signal) => ({
    expectedOrgID: props.org,
    signal,
    retry: false,
  });
  const canAct = () => isCurrent() && loaded() && !loading() && !loadError() && !pendingActionId();
  const isCurrentTarget = (target: AvailabilityTarget) =>
    targets().some((current) => unwrap(current) === unwrap(target));
  const error = createMemo(() =>
    loadError() && loaded()
      ? `${loadError()} Showing the last loaded configuration; actions are unavailable until a read succeeds.`
      : loadError(),
  );

  const rejectAccess = () => {
    if (!isCurrent()) return;
    readController?.abort();
    readController = undefined;
    batch(() => {
      setTargets([]);
      setLoaded(false);
      setLoading(false);
      setDialog(null);
      setDeleteConfirmingId(null);
      setLoadError(AVAILABILITY_ACCESS_ERROR);
    });
  };

  const loadTargets = async () => {
    if (!isCurrent() || readController) return;
    const reading = new AbortController();
    readController = reading;
    setLoading(true);
    const current = () => isCurrent() && readController === reading;
    try {
      const next = await AvailabilityTargetsAPI.list(requestOptions(reading.signal));
      if (!current()) return;
      batch(() => {
        setTargets(next);
        setLoaded(true);
        setLoadError(null);
        setDeleteConfirmingId(null);
      });
    } catch (err) {
      if (!current()) return;
      if ([401, 403].includes(apiErrorStatus(err) ?? 0)) rejectAccess();
      else setLoadError(AVAILABILITY_READ_ERROR);
    } finally {
      if (current()) {
        readController = undefined;
        setLoading(false);
      }
    }
  };
  const refreshTargets = () => {
    if (!pendingActionId()) return loadTargets();
  };

  const runAction = async (
    target: AvailabilityTarget,
    action: () => Promise<unknown>,
    removed = false,
  ) => {
    if (!canAct() || !isCurrentTarget(target)) return;
    setPendingActionId(target.id);
    try {
      await action();
      if (!isCurrent()) return;
      if (removed) {
        setDialog(null);
        setDeleteConfirmingId(null);
      }
      await loadTargets();
    } catch (err) {
      if (!isCurrent()) return;
      if ([401, 403].includes(apiErrorStatus(err) ?? 0)) rejectAccess();
      else {
        setLoadError(
          'The action could not be confirmed. Refresh saved checks before trying again.',
        );
      }
    } finally {
      if (isCurrent()) setPendingActionId(null);
    }
  };
  const toggleTarget = (target: AvailabilityTarget) =>
    runAction(target, () =>
      AvailabilityTargetsAPI.update(target.id, { enabled: !target.enabled }, requestOptions()),
    );
  const testTarget = (target: AvailabilityTarget) =>
    runAction(target, () => AvailabilityTargetsAPI.testSaved(target.id, requestOptions()));
  const requestRemoveTarget = (target: AvailabilityTarget) => {
    if (!canAct() || !isCurrentTarget(target)) return;
    if (deleteConfirmingId() !== target.id) {
      setDeleteConfirmingId(target.id);
      return;
    }
    return runAction(
      target,
      () => AvailabilityTargetsAPI.remove(target.id, requestOptions()),
      true,
    );
  };
  const openEditDialog = (target: AvailabilityTarget) => {
    if (!canAct() || !isCurrentTarget(target)) return;
    setDeleteConfirmingId(null);
    setDialog({ mode: 'edit', target });
  };
  onMount(() => void loadTargets());
  onCleanup(() => {
    disposed = true;
    controller.abort();
    readController?.abort();
  });
  return {
    targets,
    loading,
    loaded,
    loadError,
    error,
    dialog,
    setDialog,
    pendingActionId,
    deleteConfirmingId,
    setDeleteConfirmingId,
    canAct,
    isCurrent,
    rejectAccess,
    loadTargets: refreshTargets,
    toggleTarget,
    testTarget,
    requestRemoveTarget,
    openEditDialog,
  };
}
