import { Component, createSignal, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import { showSuccess } from '@/utils/toast';

interface RemovePasswordModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export const RemovePasswordModal: Component<RemovePasswordModalProps> = (props) => {
  const [currentPassword, setCurrentPassword] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal('');

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setError('');

    if (!currentPassword()) {
      setError('Current password is required');
      return;
    }

    setLoading(true);

    try {
      // Get CSRF token from cookie
      const csrfToken = document.cookie
        .split('; ')
        .find(row => row.startsWith('pulse_csrf='))
        ?.split('=')[1];

      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        'Authorization': `Basic ${btoa(`admin:${currentPassword()}`)}`,
      };
      
      // Add CSRF token if available
      if (csrfToken) {
        headers['X-CSRF-Token'] = csrfToken;
      }

      const response = await fetch('/api/security/remove-password', {
        method: 'POST',
        headers,
        body: JSON.stringify({
          currentPassword: currentPassword(),
        }),
        credentials: 'include', // Important for cookies
      });

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.message || 'Failed to remove password');
      }

      // Show success message
      showSuccess(data.message || 'Password authentication removed. Pulse is now running without authentication.');
      
      // Clear form
      setCurrentPassword('');
      props.onClose();
      
      // Reload the page to reflect the changes (password is removed from current session)
      setTimeout(() => {
        window.location.reload();
      }, 3000);
    } catch (err: any) {
      setError(err.message || 'Failed to remove password');
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setCurrentPassword('');
    setError('');
    props.onClose();
  };

  return (
    <Show when={props.isOpen}>
      <Portal>
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div class="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md w-full mx-4">
            <h2 class="text-xl font-semibold text-gray-800 dark:text-gray-200 mb-4">
              Remove Password Authentication
            </h2>

            <div class="mb-4 p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg">
              <p class="text-sm text-amber-700 dark:text-amber-300">
                <strong>Warning:</strong> This will disable password authentication. 
                Pulse will be accessible without any login. Only do this if you're on a trusted network.
              </p>
            </div>

            <form onSubmit={handleSubmit}>
              <div class="space-y-4">
                <div>
                  <label for="current-password" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Current Password
                  </label>
                  <input
                    type="password"
                    id="current-password"
                    value={currentPassword()}
                    onInput={(e) => setCurrentPassword(e.currentTarget.value)}
                    class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-gray-200"
                    placeholder="Enter current password to confirm"
                    required
                  />
                </div>

                <Show when={error()}>
                  <div class="p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
                    <p class="text-sm text-red-700 dark:text-red-300">{error()}</p>
                  </div>
                </Show>
              </div>

              <div class="flex justify-end space-x-2 mt-6">
                <button
                  type="button"
                  onClick={handleClose}
                  class="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
                  disabled={loading()}
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  class="px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
                  disabled={loading()}
                >
                  {loading() ? 'Removing...' : 'Remove Password'}
                </button>
              </div>
            </form>
          </div>
        </div>
      </Portal>
    </Show>
  );
};