import { Component, Show } from 'solid-js';
import { useWebSocket } from '@/App';
import UnifiedBackups from './UnifiedBackups';

const Backups: Component = () => {
  const { state, connected } = useWebSocket();

  return (
    <div>
      {/* Loading State */}
      <Show when={connected() && !state.pveBackups && !state.pbs}>
        <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-8">
          <div class="text-center">
            <div class="inline-flex items-center justify-center w-12 h-12 mb-4">
              <svg class="animate-spin h-8 w-8 text-blue-600 dark:text-blue-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
            </div>
            <p class="text-sm text-gray-600 dark:text-gray-400">Loading backup information...</p>
          </div>
        </div>
      </Show>

      {/* Disconnected State */}
      <Show when={!connected()}>
        <div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-600 rounded-lg p-8">
          <div class="text-center">
            <svg class="mx-auto h-12 w-12 text-red-400 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <h3 class="text-sm font-medium text-red-800 dark:text-red-200 mb-2">Connection Lost</h3>
            <p class="text-xs text-red-700 dark:text-red-300">Unable to connect to the backend server. Attempting to reconnect...</p>
          </div>
        </div>
      </Show>

      {/* Main Content - Unified Backups View */}
      <Show when={connected() && (state.pveBackups || state.pbs)}>
        <UnifiedBackups />
      </Show>
    </div>
  );
};

export default Backups;