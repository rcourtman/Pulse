import {
  batch,
  Component,
  Show,
  createEffect,
  createMemo,
  createSignal,
  on,
  onCleanup,
} from 'solid-js';
import { apiErrorFromResponse, apiFetch, getOrgID } from '@/utils/apiClient';
import type { SecurityStatus } from '@/types/config';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { Dialog } from '@/components/shared/Dialog';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';

interface ChangePasswordModalProps {
  isOpen: boolean;
  securityStatus: SecurityStatus | null;
  securityStatusLoading: boolean;
  onClose: () => void;
  onChanged: () => void;
}

export const ChangePasswordModal: Component<ChangePasswordModalProps> = (props) => {
  const [currentPassword, setCurrentPassword] = createSignal('');
  const [newPassword, setNewPassword] = createSignal('');
  const [confirmPassword, setConfirmPassword] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal('');

  const [unconfirmed, setUnconfirmed] = createSignal(false);
  const admitted = () =>
    props.securityStatus?.settingsCapabilities?.authenticationRead === true &&
    props.securityStatus?.settingsCapabilities?.authenticationWrite === true &&
    props.securityStatus?.presentationPolicy?.readOnly !== true &&
    props.securityStatus?.presentationPolicy?.demoMode !== true &&
    props.securityStatus?.sessionCapabilities?.demoMode !== true;
  // Display facts delimit this local draft's lifetime, not server authority or
  // a globally unique credential. Equal status refreshes preserve the draft.
  const reader = createMemo(() =>
    JSON.stringify([
      props.securityStatus?.currentUsername,
      props.securityStatus?.authUsername,
      props.securityStatus?.apiTokenHint,
      props.securityStatus?.tokenScopes?.slice().sort(),
      admitted(),
    ]),
  );
  const currentOrg = () => getOrgID() || 'default';
  let context: { reader: string; org: string } | undefined;
  let controller: AbortController | undefined;
  let reloadTimer: ReturnType<typeof setTimeout> | undefined;
  let disposed = false;

  const clearDraft = () => {
    context = undefined;
    controller?.abort();
    controller = undefined;
    batch(() => {
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      setLoading(false);
      setError('');
      setUnconfirmed(false);
    });
  };
  const withdraw = () => {
    clearTimeout(reloadTimer);
    clearDraft();
    if (props.isOpen) props.onClose();
  };
  createEffect(
    on([() => props.isOpen, reader], ([open, owner], previous) => {
      if (!admitted() || (previous && owner !== previous[1])) {
        withdraw();
        return;
      }
      clearDraft();
      if (open) {
        clearTimeout(reloadTimer);
        context = { reader: owner, org: currentOrg() };
      }
    }),
  );
  const unsubscribeOrg = eventBus.on('org_switched', withdraw);
  onCleanup(() => {
    disposed = true;
    unsubscribeOrg();
    clearTimeout(reloadTimer);
    clearDraft();
  });

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    const owner = context;
    if (
      disposed ||
      !owner ||
      !props.isOpen ||
      !admitted() ||
      props.securityStatusLoading ||
      loading() ||
      unconfirmed() ||
      owner.reader !== reader() ||
      owner.org !== currentOrg()
    )
      return;
    setError('');

    if (!currentPassword() || !newPassword() || !confirmPassword()) {
      setError('All fields are required');
      return;
    }
    if (newPassword() !== confirmPassword()) {
      setError('New passwords do not match');
      return;
    }
    if (newPassword().length < 12) {
      setError('Password must be at least 12 characters');
      return;
    }

    const request = new AbortController();
    controller = request;
    const isCurrent = () =>
      !disposed &&
      context === owner &&
      props.isOpen &&
      admitted() &&
      owner.reader === reader() &&
      owner.org === currentOrg() &&
      controller === request;
    setLoading(true);
    try {
      // Use only the configured local account returned by the admitted status,
      // never an old sessionStorage username or an invented admin fallback.
      const username = props.securityStatus?.authUsername;
      const headers: Record<string, string> = { 'Content-Type': 'application/json' };
      if (username) {
        const bytes = new TextEncoder().encode(`${username}:${currentPassword()}`);
        headers.Authorization = `Basic ${btoa(Array.from(bytes, (byte) => String.fromCharCode(byte)).join(''))}`;
      }
      const response = await apiFetch('/api/security/change-password', {
        method: 'POST',
        headers,
        expectedOrgID: owner.org,
        signal: request.signal,
        retry: false,
        body: JSON.stringify({ currentPassword: currentPassword(), newPassword: newPassword() }),
      });
      if (!isCurrent()) return;
      if (!response.ok) {
        if (response.status >= 500) throw new Error('Unconfirmed password change');
        const rejection = await apiErrorFromResponse(response, 'Password change was refused');
        if (!isCurrent()) return;
        const message = rejection.message;
        setError(message);
        notificationStore.error(message);
        return;
      }

      notificationStore.success(
        'Password changed successfully. Please log in with your new password.',
      );
      // Do not probe status on this handoff: the accepted password change
      // invalidates sessions, so that read would cancel its own sign-in reload.
      props.onChanged();
      clearDraft();
      // The accepted change belongs to this reader even after the dialog
      // closes. Withdrawal, reopening and disposal cancel its delayed reload.
      reloadTimer = setTimeout(() => {
        if (!disposed && admitted() && owner.reader === reader() && owner.org === currentOrg()) {
          window.location.reload();
        }
      }, 2000);
    } catch {
      if (!isCurrent()) return;
      // A missing response cannot prove whether the server applied the change.
      // Do not automatically replay it or invite another blind submission.
      setUnconfirmed(true);
      setError(
        'Password change could not be confirmed. Close this dialog and check sign-in with the new password before trying again.',
      );
    } finally {
      if (controller === request) {
        controller = undefined;
        setLoading(false);
      }
    }
  };

  const handleClose = () => {
    if (!loading()) {
      clearDraft();
      props.onClose();
    }
  };
  const fieldsDisabled = () => loading() || props.securityStatusLoading || unconfirmed();

  return (
    <Dialog
      isOpen={props.isOpen && admitted()}
      onClose={handleClose}
      panelClass="max-w-md"
      closeOnBackdrop={!loading()}
      ariaLabel="Change password"
    >
      <div class="w-full">
        <div class="flex items-center justify-between p-6 border-b border-border">
          <SectionHeader title="Change password" size="lg" class="flex-1" />
          <button
            type="button"
            onClick={handleClose}
            disabled={loading()}
            aria-label="Close password dialog"
            title="Close"
            class="text-slate-400 hover:text-muted disabled:opacity-50"
          >
            <svg
              class="w-6 h-6"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              aria-hidden="true"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>

        <form onSubmit={handleSubmit} class="p-6 space-y-4">
          <Show when={props.securityStatusLoading}>
            <p role="status" class="text-xs text-muted">
              Checking settings access… Your draft is kept while this reader is verified.
            </p>
          </Show>
          <div class={formField}>
            <label for="current-password" class={labelClass()}>
              Current Password
            </label>
            <input
              id="current-password"
              type="password"
              autocomplete="current-password"
              value={currentPassword()}
              onInput={(e) => setCurrentPassword(e.currentTarget.value)}
              class={controlClass('shadow-xs')}
              required
              disabled={fieldsDisabled()}
            />
          </div>

          <div class={formField}>
            <label for="new-password" class={labelClass()}>
              New Password
            </label>
            <input
              id="new-password"
              type="password"
              autocomplete="new-password"
              value={newPassword()}
              onInput={(e) => setNewPassword(e.currentTarget.value)}
              class={controlClass('shadow-xs')}
              required
              disabled={fieldsDisabled()}
              minLength={12}
            />
            <p class={`${formHelpText} mt-1`}>Minimum 12 characters</p>
          </div>

          <div class={formField}>
            <label for="confirm-password" class={labelClass()}>
              Confirm New Password
            </label>
            <input
              id="confirm-password"
              type="password"
              autocomplete="new-password"
              value={confirmPassword()}
              onInput={(e) => setConfirmPassword(e.currentTarget.value)}
              class={controlClass('shadow-xs')}
              required
              disabled={fieldsDisabled()}
            />
          </div>

          <Show when={error()}>
            <div
              role="alert"
              aria-live="assertive"
              class="p-3 bg-red-50 dark:bg-red-900/25 border border-red-200 dark:border-red-800 rounded-md"
            >
              <p class="text-sm text-red-600 dark:text-red-400">{error()}</p>
            </div>
          </Show>

          <div class="flex justify-end space-x-3 pt-4">
            <button
              type="button"
              onClick={handleClose}
              disabled={loading()}
              class="px-4 py-2 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={fieldsDisabled()}
              class="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md disabled:opacity-50"
            >
              {loading() ? 'Changing...' : 'Change Password'}
            </button>
          </div>
        </form>
      </div>
    </Dialog>
  );
};
