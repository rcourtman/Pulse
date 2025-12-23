import { Component, createSignal, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import { apiFetch } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';

interface ChangePasswordModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export const ChangePasswordModal: Component<ChangePasswordModalProps> = (props) => {
  const [currentPassword, setCurrentPassword] = createSignal('');
  const [newPassword, setNewPassword] = createSignal('');
  const [confirmPassword, setConfirmPassword] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal('');

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setError('');

    // Validation
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

    setLoading(true);

    try {
      // Get the actual username from sessionStorage or use 'admin' as fallback
      const authUser = sessionStorage.getItem('pulse_auth_user') || 'admin';

      const response = await apiFetch('/api/security/change-password', {
        method: 'POST',
        headers: {
          Authorization: `Basic ${btoa(`${authUser}:${currentPassword()}`)}`,
        },
        body: JSON.stringify({
          currentPassword: currentPassword(),
          newPassword: newPassword(),
        }),
      });

      if (!response.ok) {
        const text = await response.text();
        if (response.status === 401) {
          throw new Error('Current password is incorrect');
        }
        throw new Error(text || 'Failed to change password');
      }

      notificationStore.success('Password changed successfully. Please log in with your new password.');

      // Clear form
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');

      // Close modal and trigger re-authentication
      props.onClose();

      // Reload page to force re-login with new password
      setTimeout(() => {
        window.location.reload();
      }, 2000);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to change password';
      setError(errorMessage);
      notificationStore.error(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    if (!loading()) {
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      setError('');
      props.onClose();
    }
  };

  return (
    <Show when={props.isOpen}>
      <Portal>
        <div
          class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4"
          style="z-index: 9999"
        >
          <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full">
            <div class="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
              <SectionHeader title="Change password" size="lg" class="flex-1" />
              <button
                type="button"
                onClick={handleClose}
                disabled={loading()}
                class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 disabled:opacity-50"
              >
                <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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
              <div class={formField}>
                <label for="current-password" class={labelClass()}>
                  Current Password
                </label>
                <input
                  id="current-password"
                  type="password"
                  value={currentPassword()}
                  onInput={(e) => setCurrentPassword(e.currentTarget.value)}
                  class={controlClass('shadow-sm')}
                  required
                  disabled={loading()}
                />
              </div>

              <div class={formField}>
                <label for="new-password" class={labelClass()}>
                  New Password
                </label>
                <input
                  id="new-password"
                  type="password"
                  value={newPassword()}
                  onInput={(e) => setNewPassword(e.currentTarget.value)}
                  class={controlClass('shadow-sm')}
                  required
                  disabled={loading()}
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
                  value={confirmPassword()}
                  onInput={(e) => setConfirmPassword(e.currentTarget.value)}
                  class={controlClass('shadow-sm')}
                  required
                  disabled={loading()}
                />
              </div>

              <Show when={error()}>
                <div class="p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md">
                  <p class="text-sm text-red-600 dark:text-red-400">{error()}</p>
                </div>
              </Show>

              <div class="flex justify-end space-x-3 pt-4">
                <button
                  type="button"
                  onClick={handleClose}
                  disabled={loading()}
                  class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={loading()}
                  class="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md disabled:opacity-50"
                >
                  {loading() ? 'Changing...' : 'Change Password'}
                </button>
              </div>
            </form>
          </div>
        </div>
      </Portal>
    </Show>
  );
};
