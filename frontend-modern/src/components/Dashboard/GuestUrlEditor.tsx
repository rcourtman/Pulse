import { Component, createSignal, Show } from 'solid-js';
import { GuestMetadataAPI } from '@/api/guestMetadata';

interface GuestUrlEditorProps {
  guestId: string;
  guestName: string;
  currentUrl?: string;
  onUpdate: (url: string | undefined) => void;
  onClose: () => void;
}

export const GuestUrlEditor: Component<GuestUrlEditorProps> = (props) => {
  const [url, setUrl] = createSignal(props.currentUrl || '');
  const [saving, setSaving] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);

  const handleSave = async () => {
    const urlValue = url().trim();
    
    // Validate URL if provided
    if (urlValue && !urlValue.startsWith('http://') && !urlValue.startsWith('https://')) {
      setError('URL must start with http:// or https://');
      return;
    }

    setSaving(true);
    setError(null);

    try {
      await GuestMetadataAPI.updateMetadata(props.guestId, {
        customUrl: urlValue || undefined
      });
      props.onUpdate(urlValue || undefined);
      // onClose is handled by onUpdate in Dashboard
    } catch (err) {
      setError('Failed to save URL');
      console.error('Failed to save guest URL:', err);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    setSaving(true);
    setError(null);

    try {
      await GuestMetadataAPI.updateMetadata(props.guestId, {
        customUrl: undefined
      });
      props.onUpdate(undefined);
      // onClose is handled by onUpdate in Dashboard
    } catch (err) {
      setError('Failed to remove URL');
      console.error('Failed to remove guest URL:', err);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl p-6 max-w-md w-full mx-4">
        <h3 class="text-lg font-semibold mb-4 text-gray-900 dark:text-gray-100">
          Edit URL for {props.guestName}
        </h3>
        
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Custom URL
            </label>
            <input
              type="url"
              value={url()}
              onInput={(e) => setUrl(e.currentTarget.value)}
              placeholder="https://example.com"
              class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-gray-700 dark:text-gray-100"
            />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Leave empty to remove the custom URL
            </p>
          </div>

          <Show when={error()}>
            <div class="text-sm text-red-600 dark:text-red-400">
              {error()}
            </div>
          </Show>

          <div class="flex justify-end gap-2">
            <Show when={props.currentUrl}>
              <button
                onClick={handleDelete}
                disabled={saving()}
                class="px-4 py-2 text-sm font-medium text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 disabled:opacity-50"
              >
                Remove URL
              </button>
            </Show>
            <button
              onClick={props.onClose}
              disabled={saving()}
              class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded-md disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              disabled={saving()}
              class="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md disabled:opacity-50"
            >
              {saving() ? 'Saving...' : 'Save'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};