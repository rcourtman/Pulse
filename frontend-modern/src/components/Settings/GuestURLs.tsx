import { createSignal, createMemo, onMount, For, Show } from 'solid-js';
import { useWebSocket } from '@/App';
import { showSuccess, showError } from '@/utils/toast';
import type { VM, Container } from '@/types/api';
import { GuestMetadataAPI } from '@/api/guestMetadata';
import type { GuestMetadata } from '@/api/guestMetadata';
import { SectionHeader } from '@/components/shared/SectionHeader';


interface GuestURLsProps {
  hasUnsavedChanges: () => boolean;
  setHasUnsavedChanges: (value: boolean) => void;
}

export function GuestURLs(props: GuestURLsProps) {
  const { state } = useWebSocket();
  const [guestMetadata, setGuestMetadata] = createSignal<Record<string, GuestMetadata>>({});
  const [searchTerm, setSearchTerm] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [initialLoad, setInitialLoad] = createSignal(true);
  const [urlErrors, setUrlErrors] = createSignal<Record<string, string>>({});

  // Combine VMs and containers into a single list
  const allGuests = createMemo(() => {
    const vms = (state.vms || []) as VM[];
    const containers = (state.containers || []) as Container[];
    return [...vms, ...containers];
  });

  // Filter and group guests by node
  const groupedGuests = createMemo(() => {
    const search = searchTerm().toLowerCase();
    let guests = allGuests();
    
    // Apply search filter
    if (search) {
      guests = guests.filter(guest => 
        guest.name.toLowerCase().includes(search) ||
        guest.vmid.toString().includes(search) ||
        guest.node.toLowerCase().includes(search)
      );
    }
    
    // Group by node
    const groups: Record<string, (VM | Container)[]> = {};
    guests.forEach(guest => {
      if (!groups[guest.node]) {
        groups[guest.node] = [];
      }
      groups[guest.node].push(guest);
    });
    
    // Sort guests within each node by VMID
    Object.keys(groups).forEach(node => {
      groups[node] = groups[node].sort((a, b) => a.vmid - b.vmid);
    });
    
    return groups;
  });

  // Load saved URLs from backend on mount
  onMount(async () => {
    setLoading(true);
    try {
      const metadata = await GuestMetadataAPI.getAllMetadata();
      setGuestMetadata(metadata || {});
    } catch (err) {
      console.error('Failed to load guest metadata:', err);
      showError('Failed to load guest URLs');
    } finally {
      setLoading(false);
      setInitialLoad(false);
    }
  });

  // Save URLs to backend
  const saveURLs = async () => {
    setLoading(true);
    try {
      const metadata = guestMetadata();
      const errors: string[] = [];
      
      // Update each guest that has changes
      for (const [guestId, meta] of Object.entries(metadata)) {
        if (meta.customUrl !== undefined) {
          try {
            await GuestMetadataAPI.updateMetadata(guestId, { customUrl: meta.customUrl });
          } catch (err: any) {
            // Extract error message from response
            const errorMsg = err.message || err.toString();
            errors.push(`${guestId}: ${errorMsg}`);
            console.error(`Failed to save URL for ${guestId}:`, err);
          }
        }
      }
      
      if (errors.length > 0) {
        // Show specific validation errors
        showError(errors.join('\n'));
      } else {
        showSuccess('Guest URLs saved');
        props.setHasUnsavedChanges(false);
      }
    } catch (err) {
      console.error('Failed to save guest URLs:', err);
      showError('Failed to save guest URLs');
    } finally {
      setLoading(false);
    }
  };

  // Validate URL format
  const validateURL = (url: string): string | null => {
    if (!url) return null; // Empty is valid
    
    // Check for incomplete URLs like "https://emby."
    if (url.endsWith('.') && !url.includes('..')) {
      return 'URL appears incomplete - please enter a complete domain or IP address';
    }
    
    // Check for missing protocol
    if (!url.startsWith('http://') && !url.startsWith('https://')) {
      return 'URL must start with http:// or https://';
    }
    
    try {
      const parsed = new URL(url);
      
      // Check for valid host
      if (!parsed.hostname) {
        return 'URL must include a valid hostname or IP address';
      }
      
      // Check for incomplete hostnames
      if (parsed.hostname.endsWith('.') && !parsed.hostname.includes('..')) {
        return 'Hostname appears incomplete';
      }
      
      return null; // Valid
    } catch (e) {
      return 'Invalid URL format';
    }
  };

  // Resolve the metadata key for a guest, supporting legacy identifiers
  const resolveMetadataKey = (guestId: string, fallbackId: string) => {
    const metadata = guestMetadata();
    if (metadata[guestId]) return guestId;
    if (metadata[fallbackId]) return fallbackId;
    return guestId;
  };

  // Update a guest's URL configuration
  const updateGuestURL = (guestId: string, url: string) => {
    setGuestMetadata(prev => ({
      ...prev,
      [guestId]: {
        ...(prev[guestId] || { id: guestId }),
        customUrl: url
      }
    }));

    const error = validateURL(url);
    setUrlErrors(prev => {
      const next = { ...prev };
      if (error) {
        next[guestId] = error;
      } else {
        delete next[guestId];
      }
      return next;
    });

    props.setHasUnsavedChanges(true);
  };

  // Clear a guest's URL configuration
  const clearGuestURL = (guestId: string) => {
    setGuestMetadata(prev => ({
      ...prev,
      [guestId]: {
        ...(prev[guestId] || { id: guestId }),
        customUrl: ''
      }
    }));

    setUrlErrors(prev => {
      const next = { ...prev };
      delete next[guestId];
      return next;
    });

    props.setHasUnsavedChanges(true);
  };

  return (
    <div class="space-y-6">
      {/* Header */}
      <SectionHeader
        title="Guest URL management"
        description="Configure custom URLs for accessing guest web interfaces. These URLs appear as shortcuts from the dashboard."
        size="md"
      />

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
      <Show when={props.hasUnsavedChanges() && Object.keys(urlErrors()).length === 0}>
        <div class="flex justify-end">
          <button type="button"
            onClick={saveURLs}
            disabled={loading()}
            class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading() ? 'Saving...' : 'Save Changes'}
          </button>
        </div>
      </Show>

      {/* Guest URLs Table */}
      <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        <Show when={!initialLoad()} fallback={
          <div class="flex items-center justify-center py-12">
            <div class="text-gray-500 dark:text-gray-400">Loading guest URLs...</div>
          </div>
        }>
          <div class="overflow-x-auto" style="scrollbar-width: none; -ms-overflow-style: none;">
            <style>{`
              .overflow-x-auto::-webkit-scrollbar { display: none; }
            `}</style>
            <table class="min-w-full" style="min-width: 768px;">
              <thead>
                <tr class="bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700">
                  <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-32">
                    Name
                  </th>
                  <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-16">
                    Type
                  </th>
                  <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-16">
                    VMID
                  </th>
                  <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider" style="min-width: 350px;">
                    Custom URL
                  </th>
                  <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-20">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                <Show when={Object.keys(groupedGuests()).length === 0} fallback={
                <For each={Object.entries(groupedGuests()).sort(([a], [b]) => a.localeCompare(b))}>
                  {([node, guests]) => (
                    <>
                      {/* Node header row */}
                      <tr class="node-header bg-gray-50 dark:bg-gray-700/50 font-semibold text-gray-700 dark:text-gray-300 text-xs">
                        <td colspan="5" class="px-3 py-1 text-xs font-medium text-gray-500 dark:text-gray-400">
                          {node}
                        </td>
                      </tr>
                      {/* Guest rows for this node */}
                      <For each={guests}>
                        {(guest) => {
                          const guestId = guest.id || `${guest.instance}-${guest.node}-${guest.vmid}`;
                          const fallbackId = `${guest.node}-${guest.vmid}`;
                          const metadataKey = resolveMetadataKey(guestId, fallbackId);
                          const meta = guestMetadata()[metadataKey];
                          const url = meta?.customUrl;
                          const urlError = urlErrors()[metadataKey];
                          
                          return (
                            <tr class="hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-colors">
                              <td class="p-1 px-2">
                                <div class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                  {guest.name}
                                </div>
                              </td>
                              <td class="p-1 px-2">
                                <span class={`inline-block px-1.5 py-0.5 text-xs font-medium rounded ${
                                  guest.type === 'qemu' 
                                    ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300' 
                                    : 'bg-green-100 text-green-700 dark:bg-green-900/50 dark:text-green-300'
                                }`}>
                                  {guest.type === 'qemu' ? 'VM' : 'LXC'}
                                </span>
                              </td>
                              <td class="p-1 px-2 text-sm text-gray-600 dark:text-gray-400">
                                {guest.vmid}
                              </td>
                              <td class="p-1 px-2">
                                <div>
                                  <input
                                    type="text"
                                    placeholder="https://192.168.1.100:8006"
                                    value={meta?.customUrl || ''}
                                    onInput={(e) => updateGuestURL(metadataKey, e.currentTarget.value)}
                                    class={`w-full min-w-[300px] px-2 py-1 text-sm border rounded
                                           bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100
                                           focus:ring-2 focus:border-transparent ${
                                           urlError 
                                             ? 'border-red-500 dark:border-red-400 focus:ring-red-500' 
                                             : 'border-gray-300 dark:border-gray-600 focus:ring-blue-500'
                                           }`}
                                    style="min-width: 300px;"
                                  />
                                  <Show when={urlError}>
                                    <div class="text-xs text-red-600 dark:text-red-400 mt-1">
                                      {urlError}
                                    </div>
                                  </Show>
                                </div>
                              </td>
                              <td class="p-1 px-2">
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
                                  <Show when={meta?.customUrl}>
                                    <button type="button"
                                      onClick={() => clearGuestURL(metadataKey)}
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
                    </>
                  )}
                </For>
              }>
                <tr>
                  <td colspan="5" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
                    No guests found
                  </td>
                </tr>
                </Show>
              </tbody>
            </table>
          </div>
        </Show>
      </div>
    </div>
  );
}
