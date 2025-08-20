import { createSignal, createMemo, onMount, For, Show } from 'solid-js';
import { useWebSocket } from '@/App';
import { showSuccess } from '@/utils/toast';
import type { VM, Container } from '@/types/api';

interface GuestURL {
  guestId: string;
  url: string;
}

interface GuestURLsProps {
  hasUnsavedChanges: () => boolean;
  setHasUnsavedChanges: (value: boolean) => void;
}

export function GuestURLs(props: GuestURLsProps) {
  const { state } = useWebSocket();
  const [guestURLs, setGuestURLs] = createSignal<Record<string, GuestURL>>({});
  const [searchTerm, setSearchTerm] = createSignal('');

  // Combine VMs and containers into a single list
  const allGuests = createMemo(() => {
    const vms = (state.vms || []) as VM[];
    const containers = (state.containers || []) as Container[];
    const combined = [...vms, ...containers];
    
    // Sort by name
    return combined.sort((a, b) => a.name.localeCompare(b.name));
  });

  // Filter guests based on search
  const filteredGuests = createMemo(() => {
    const search = searchTerm().toLowerCase();
    if (!search) return allGuests();
    
    return allGuests().filter(guest => 
      guest.name.toLowerCase().includes(search) ||
      guest.vmid.toString().includes(search) ||
      guest.node.toLowerCase().includes(search)
    );
  });

  // Load saved URLs on mount
  onMount(() => {
    const savedURLs = localStorage.getItem('guestURLs');
    if (savedURLs) {
      try {
        setGuestURLs(JSON.parse(savedURLs));
      } catch (err) {
        console.error('Failed to parse saved URLs:', err);
      }
    }
  });

  // Save URLs to localStorage whenever they change
  const saveURLs = () => {
    localStorage.setItem('guestURLs', JSON.stringify(guestURLs()));
    showSuccess('Guest URLs saved');
    props.setHasUnsavedChanges(false);
  };

  // Update a guest's URL configuration
  const updateGuestURL = (guestId: string, url: string) => {
    setGuestURLs({
      ...guestURLs(),
      [guestId]: {
        guestId,
        url
      }
    });
    
    props.setHasUnsavedChanges(true);
  };

  // Clear a guest's URL configuration
  const clearGuestURL = (guestId: string) => {
    const updated = { ...guestURLs() };
    delete updated[guestId];
    setGuestURLs(updated);
    props.setHasUnsavedChanges(true);
  };

  // Get the URL for a guest
  const getURL = (guestId: string): string | undefined => {
    const config = guestURLs()[guestId];
    return config?.url || undefined;
  };

  return (
    <div class="space-y-6">
      {/* Header */}
      <div>
        <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-2">Guest URL Management</h3>
        <p class="text-sm text-gray-600 dark:text-gray-400">
          Configure custom URLs for accessing guest web interfaces. These URLs will be clickable from the dashboard.
        </p>
      </div>

      {/* Search */}
      <div class="relative">
        <input
          type="text"
          placeholder="Search guests..."
          value={searchTerm()}
          onInput={(e) => setSearchTerm(e.currentTarget.value)}
          class="w-full px-4 py-2 pl-10 text-sm border border-gray-300 dark:border-gray-600 rounded-lg 
                 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100
                 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
        />
        <svg class="absolute left-3 top-2.5 w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
      </div>

      {/* Save Button */}
      <Show when={props.hasUnsavedChanges()}>
        <div class="flex justify-end">
          <button
            onClick={saveURLs}
            class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 transition-colors"
          >
            Save Changes
          </button>
        </div>
      </Show>

      {/* Guest URLs Table */}
      <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div class="overflow-x-auto">
          <table class="w-full">
            <thead>
              <tr class="bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700">
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Guest
                </th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Custom URL
                </th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
              <For each={filteredGuests()} fallback={
                <tr>
                  <td colspan="3" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
                    No guests found
                  </td>
                </tr>
              }>
                {(guest) => {
                  const guestId = guest.id || `${guest.instance}-${guest.name}-${guest.vmid}`;
                  const config = guestURLs()[guestId];
                  const url = getURL(guestId);
                  
                  return (
                    <tr class="hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-colors">
                      <td class="px-4 py-3">
                        <div class="flex items-center gap-2">
                          <div>
                            <div class="text-sm font-medium text-gray-900 dark:text-gray-100">
                              {guest.name}
                            </div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">
                              {guest.type === 'qemu' ? 'VM' : 'LXC'} • {guest.vmid} • {guest.node}
                            </div>
                          </div>
                        </div>
                      </td>
                      <td class="px-4 py-3">
                        <input
                          type="text"
                          placeholder="https://192.168.1.100:8006 or http://example.com"
                          value={config?.url || ''}
                          onInput={(e) => updateGuestURL(guestId, e.currentTarget.value)}
                          class="w-full px-2 py-1 text-sm border border-gray-300 dark:border-gray-600 rounded
                                 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100
                                 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        />
                      </td>
                      <td class="px-4 py-3">
                        <div class="flex items-center gap-2">
                          <Show when={url}>
                            <a
                              href={url}
                              target="_blank"
                              rel="noopener noreferrer"
                              class="text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                              title="Test URL"
                            >
                              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" 
                                  d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                              </svg>
                            </a>
                          </Show>
                          <Show when={config?.url}>
                            <button
                              onClick={() => clearGuestURL(guestId)}
                              class="text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
                              title="Clear URL"
                            >
                              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" 
                                  d="M6 18L18 6M6 6l12 12" />
                              </svg>
                            </button>
                          </Show>
                        </div>
                      </td>
                    </tr>
                  );
                }}
              </For>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}