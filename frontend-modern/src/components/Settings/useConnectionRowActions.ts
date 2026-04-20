import { createSignal, onCleanup } from 'solid-js';
import { ConnectionsAPI, type Connection } from '@/api/connections';

const REMOVE_CONFIRM_TIMEOUT_MS = 6000;

type PendingAction = 'pause' | 'remove';

const errorMessage = (err: unknown): string => {
  if (err instanceof Error && err.message) return err.message;
  if (typeof err === 'string' && err.trim()) return err;
  return 'Something went wrong.';
};

export interface ConnectionRowActionsOptions {
  onMutated?: () => void;
}

export interface ConnectionRowActions {
  pendingAction: (id: string) => PendingAction | null;
  actionError: (id: string) => string | null;
  confirmingRemove: (id: string) => boolean;
  togglePause: (connection: Connection) => Promise<void>;
  requestRemove: (connection: Connection) => Promise<void>;
  cancelRemove: (id: string) => void;
}

export const useConnectionRowActions = (
  options: ConnectionRowActionsOptions = {},
): ConnectionRowActions => {
  const [pending, setPending] = createSignal<Record<string, PendingAction | null>>({});
  const [errors, setErrors] = createSignal<Record<string, string | null>>({});
  const [confirming, setConfirming] = createSignal<Record<string, boolean>>({});
  const timers = new Map<string, number>();

  const clearTimer = (id: string) => {
    const handle = timers.get(id);
    if (handle !== undefined) {
      window.clearTimeout(handle);
      timers.delete(id);
    }
  };

  onCleanup(() => {
    timers.forEach((handle) => window.clearTimeout(handle));
    timers.clear();
  });

  const setPendingFor = (id: string, value: PendingAction | null) =>
    setPending((prev) => ({ ...prev, [id]: value }));
  const setErrorFor = (id: string, value: string | null) =>
    setErrors((prev) => ({ ...prev, [id]: value }));
  const setConfirmingFor = (id: string, value: boolean) =>
    setConfirming((prev) => ({ ...prev, [id]: value }));

  const togglePause = async (connection: Connection) => {
    const id = connection.id;
    setErrorFor(id, null);
    setPendingFor(id, 'pause');
    try {
      await ConnectionsAPI.setEnabled(id, !connection.enabled);
      options.onMutated?.();
    } catch (err) {
      setErrorFor(id, errorMessage(err));
    } finally {
      setPendingFor(id, null);
    }
  };

  const cancelRemove = (id: string) => {
    clearTimer(id);
    setConfirmingFor(id, false);
  };

  const requestRemove = async (connection: Connection) => {
    const id = connection.id;
    setErrorFor(id, null);
    if (!confirming()[id]) {
      setConfirmingFor(id, true);
      clearTimer(id);
      const handle = window.setTimeout(() => {
        setConfirmingFor(id, false);
        timers.delete(id);
      }, REMOVE_CONFIRM_TIMEOUT_MS);
      timers.set(id, handle);
      return;
    }
    clearTimer(id);
    setConfirmingFor(id, false);
    setPendingFor(id, 'remove');
    try {
      await ConnectionsAPI.remove(id);
      options.onMutated?.();
    } catch (err) {
      setErrorFor(id, errorMessage(err));
    } finally {
      setPendingFor(id, null);
    }
  };

  return {
    pendingAction: (id) => pending()[id] ?? null,
    actionError: (id) => errors()[id] ?? null,
    confirmingRemove: (id) => Boolean(confirming()[id]),
    togglePause,
    requestRemove,
    cancelRemove,
  };
};
