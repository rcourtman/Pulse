import { Component, Show, createSignal, For, createEffect } from 'solid-js';
import { Portal } from 'solid-js/web';
import { showSuccess, showError } from '@/utils/toast';
import { SectionHeader } from '@/components/shared/SectionHeader';

interface DiscoveredServer {
  ip: string;
  port: number;
  type: 'pve' | 'pbs';
  version: string;
  hostname?: string;
  release?: string;
}

interface DiscoveryResult {
  servers: DiscoveredServer[];
  errors?: string[];
}

interface DiscoveryModalProps {
  isOpen: boolean;
  onClose: () => void;
  onAddServers: (servers: DiscoveredServer[]) => void;
}

export const DiscoveryModal: Component<DiscoveryModalProps> = (props) => {
  const [isScanning, setIsScanning] = createSignal(false);
  const [subnet, setSubnet] = createSignal('auto');
  const [discoveryResult, setDiscoveryResult] = createSignal<DiscoveryResult | null>(null);
  const [hasScanned, setHasScanned] = createSignal(false);

  // Load cached results when modal opens
  createEffect(() => {
    if (props.isOpen && !hasScanned()) {
      setHasScanned(true);
      loadCachedResults();
    }
  });

  // Reset scan state when modal closes
  createEffect(() => {
    if (!props.isOpen) {
      setHasScanned(false);
      // Keep cached results, don't clear them
    }
  });

  // Listen for real-time WebSocket updates when modal is open and scanning
  createEffect(() => {
    if (!props.isOpen) return;

    const handleWsMessage = (event: MessageEvent) => {
      try {
        const message = JSON.parse(event.data);

        // Handle discovery server found (real-time updates)
        if (message.type === 'discovery_server_found' && message.data?.server) {
          const server: DiscoveredServer = message.data.server;

          // Add server to results immediately
          setDiscoveryResult((prev) => {
            if (!prev) {
              return { servers: [server], errors: [] };
            }

            // Check if server already exists (by IP and port)
            const exists = prev.servers.some(
              (s) => s.ip === server.ip && s.port === server.port
            );

            if (!exists) {
              return {
                ...prev,
                servers: [...prev.servers, server],
              };
            }
            return prev;
          });
        }

        // Handle discovery started
        if (message.type === 'discovery_started') {
          setIsScanning(true);
        }

        // Handle discovery complete
        if (message.type === 'discovery_complete') {
          setIsScanning(false);
        }
      } catch (error) {
        console.error('Error parsing WebSocket message:', error);
      }
    };

    // Get WebSocket from global state
    const ws = (window as any).__pulseWs;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.addEventListener('message', handleWsMessage);

      return () => {
        ws.removeEventListener('message', handleWsMessage);
      };
    }
  });

  const loadCachedResults = async () => {
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      // Fetch cached results with GET request
      const response = await apiFetch('/api/discover', {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (response.ok) {
        const data = await response.json();

        // If we have cached results, show them immediately
        if (data.servers && data.servers.length > 0) {
          setDiscoveryResult({
            servers: data.servers,
            errors: data.errors || [],
          });
        } else {
          // No cached results, start a background scan
          handleScan();
        }
      } else {
        // Fallback to scanning
        handleScan();
      }
    } catch (error) {
      console.error('Failed to load cached discovery results:', error);
      // Fallback to scanning
      handleScan();
    }
  };

  const handleRefresh = async () => {
    // First try to get cached results immediately
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/discover', {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (response.ok) {
        const data = await response.json();

        // If we have cached results, show them immediately
        if (data.servers && data.servers.length > 0) {
          setDiscoveryResult({
            servers: data.servers,
            errors: data.errors || [],
          });
          showSuccess(`Showing ${data.servers.length} cached server(s)`);
          return; // Don't start a new scan
        }
      }
    } catch (error) {
      console.error('Failed to load cached results:', error);
    }

    // If no cached results or error, start a new scan
    handleScan();
  };

  const handleScan = async () => {
    setIsScanning(true);
    setDiscoveryResult(null);

    // Set a timeout for the scan (30 seconds)
    const controller = new AbortController();
    const timeoutId = setTimeout(() => {
      controller.abort();
    }, 30000);

    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/discover', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ subnet: subnet() }),
        signal: controller.signal,
      });

      clearTimeout(timeoutId);

      if (!response.ok) {
        throw new Error(await response.text());
      }

      const result: DiscoveryResult = await response.json();
      setDiscoveryResult(result);

      if (result.servers.length === 0) {
        showError('No Proxmox/PBS servers found on the network');
      } else {
        showSuccess(`Found ${result.servers.length} server(s)`);
      }
    } catch (error) {
      clearTimeout(timeoutId);

      if (error instanceof Error && error.name === 'AbortError') {
        showError('Scan timeout - try a smaller subnet range');
      } else {
        showError(`Discovery failed: ${error instanceof Error ? error.message : 'Unknown error'}`);
      }
    } finally {
      setIsScanning(false);
    }
  };

  const handleAddServer = (server: DiscoveredServer) => {
    props.onAddServers([server]);
  };

  const getServerIcon = (type: string) => {
    if (type === 'pve') {
      return (
        <svg
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <rect x="4" y="4" width="16" height="16" rx="2" ry="2"></rect>
          <rect x="9" y="9" width="6" height="6"></rect>
          <line x1="9" y1="1" x2="9" y2="4"></line>
          <line x1="15" y1="1" x2="15" y2="4"></line>
          <line x1="9" y1="20" x2="9" y2="23"></line>
          <line x1="15" y1="20" x2="15" y2="23"></line>
          <line x1="20" y1="9" x2="23" y2="9"></line>
          <line x1="20" y1="14" x2="23" y2="14"></line>
          <line x1="1" y1="9" x2="4" y2="9"></line>
          <line x1="1" y1="14" x2="4" y2="14"></line>
        </svg>
      );
    }
    return (
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 01-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 011-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 011.52 0C14.51 3.81 17 5 19 5a1 1 0 011 1v7z"></path>
        <path d="M9 12l2 2 4-4"></path>
      </svg>
    );
  };

  return (
    <Portal>
      <Show when={props.isOpen}>
        <div class="fixed inset-0 z-50 overflow-y-auto">
          <div class="flex min-h-screen items-center justify-center p-4">
            {/* Backdrop */}
            <div class="fixed inset-0 bg-black/50 transition-opacity" onClick={props.onClose} />

            {/* Modal */}
            <div class="relative w-full max-w-3xl bg-white dark:bg-gray-800 rounded-lg shadow-xl">
              {/* Header */}
              <div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
                <SectionHeader title="Network discovery" size="md" class="flex-1" />
                <button
                  type="button"
                  onClick={props.onClose}
                  class="text-gray-400 hover:text-gray-500 dark:hover:text-gray-300"
                >
                  <svg
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <line x1="18" y1="6" x2="6" y2="18"></line>
                    <line x1="6" y1="6" x2="18" y2="18"></line>
                  </svg>
                </button>
              </div>

              {/* Body */}
              <div class="p-6">
                {/* Quick Actions Bar */}
                <div class="flex justify-between items-center mb-6">
                  <div class="flex items-center gap-3">
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">
                      <Show when={isScanning()} fallback="Network Servers">
                        Scanning Network...
                      </Show>
                    </h4>
                    <Show when={discoveryResult() && !isScanning()}>
                      <span class="text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 px-2 py-1 rounded-full">
                        {discoveryResult()!.servers.length} found
                      </span>
                    </Show>
                  </div>

                  <div class="flex items-center gap-2">
                    {/* Subnet selector */}
                    <select
                      value={subnet()}
                      onChange={(e) => {
                        setSubnet(e.currentTarget.value);
                        setHasScanned(false);
                        handleScan();
                      }}
                      class="px-3 py-1.5 text-xs border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                      disabled={isScanning()}
                    >
                      <option value="auto">Auto-detect</option>
                      <option value="192.168.1.0/24">192.168.1.x</option>
                      <option value="192.168.0.0/24">192.168.0.x</option>
                      <option value="10.0.0.0/24">10.0.0.x</option>
                      <option value="172.16.0.0/24">172.16.0.x</option>
                    </select>

                    {/* Refresh button */}
                    <button
                      type="button"
                      onClick={handleRefresh}
                      disabled={isScanning()}
                      title="Refresh scan"
                      class="p-1.5 text-sm border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      <Show
                        when={isScanning()}
                        fallback={
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <polyline points="23 4 23 10 17 10"></polyline>
                            <polyline points="1 20 1 14 7 14"></polyline>
                            <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
                          </svg>
                        }
                      >
                        <svg class="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
                          <circle
                            class="opacity-25"
                            cx="12"
                            cy="12"
                            r="10"
                            stroke="currentColor"
                            stroke-width="4"
                          ></circle>
                          <path
                            class="opacity-75"
                            fill="currentColor"
                            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                          ></path>
                        </svg>
                      </Show>
                    </button>
                  </div>
                </div>

                {/* Show loading message when scanning with no results yet */}
                <Show when={isScanning() && !discoveryResult()}>
                  <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                    <svg
                      class="animate-spin h-12 w-12 mx-auto mb-4 text-blue-500"
                      viewBox="0 0 24 24"
                      fill="none"
                    >
                      <circle
                        class="opacity-25"
                        cx="12"
                        cy="12"
                        r="10"
                        stroke="currentColor"
                        stroke-width="4"
                      ></circle>
                      <path
                        class="opacity-75"
                        fill="currentColor"
                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                      ></path>
                    </svg>
                    <p>Scanning network...</p>
                    <p class="text-xs mt-2">Servers will appear here as they're discovered</p>
                  </div>
                </Show>

                {/* Discovery Results - show even while scanning */}
                <Show when={discoveryResult()}>
                  <div class="space-y-4">
                    {/* Server Cards */}
                    <Show
                      when={discoveryResult()!.servers.length > 0}
                      fallback={
                        <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                          <svg
                            width="48"
                            height="48"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="1.5"
                            class="mx-auto mb-3 opacity-50"
                          >
                            <circle cx="11" cy="11" r="8"></circle>
                            <path d="m21 21-4.35-4.35"></path>
                            <path d="M11 8v3m0 4h.01"></path>
                          </svg>
                          <p>No Proxmox servers found on this network</p>
                          <p class="text-xs mt-2">
                            Try a different subnet or check if servers are online
                          </p>
                        </div>
                      }
                    >
                      <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                        <For each={discoveryResult()!.servers}>
                          {(server) => (
                            <div
                              class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 cursor-pointer transition-all hover:shadow-md hover:border-blue-300 dark:hover:border-blue-600 hover:bg-gray-50 dark:hover:bg-gray-700/30 group"
                              onClick={() => handleAddServer(server)}
                            >
                              <div class="flex items-start justify-between">
                                <div class="flex items-start gap-3">
                                  {/* Server Icon */}
                                  <div
                                    class={`mt-0.5 ${server.type === 'pve' ? 'text-orange-500' : 'text-green-500'}`}
                                  >
                                    {getServerIcon(server.type)}
                                  </div>

                                  {/* Server Details */}
                                  <div class="flex-1">
                                    <div class="font-medium text-gray-900 dark:text-gray-100">
                                      {server.hostname || server.ip}
                                    </div>
                                    <div class="text-sm text-gray-500 dark:text-gray-400 mt-1">
                                      {server.ip}:{server.port}
                                    </div>
                                    <div class="flex items-center gap-2 mt-2">
                                      <span class="text-xs px-2 py-0.5 rounded-full bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400">
                                        {server.type === 'pve' ? 'Proxmox VE' : 'PBS'}
                                      </span>
                                      <Show when={server.version !== 'Unknown'}>
                                        <span class="text-xs text-gray-500 dark:text-gray-400">
                                          v{server.version}
                                        </span>
                                      </Show>
                                    </div>
                                  </div>
                                </div>

                                {/* Add Arrow */}
                                <div class="text-gray-400 group-hover:text-blue-500 transition-colors">
                                  <svg
                                    width="20"
                                    height="20"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    stroke-width="2"
                                  >
                                    <line x1="12" y1="5" x2="12" y2="19"></line>
                                    <line x1="5" y1="12" x2="19" y2="12"></line>
                                  </svg>
                                </div>
                              </div>
                            </div>
                          )}
                        </For>
                      </div>
                    </Show>

                    {/* Errors */}
                    <Show when={discoveryResult()!.errors && discoveryResult()!.errors!.length > 0}>
                      <div class="mt-4 p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg">
                        <h5 class="text-sm font-medium text-yellow-800 dark:text-yellow-200 mb-2">
                          Discovery Warnings
                        </h5>
                        <ul class="text-xs text-yellow-700 dark:text-yellow-300 space-y-1">
                          <For each={discoveryResult()!.errors}>
                            {(error) => <li>• {error}</li>}
                          </For>
                        </ul>
                      </div>
                    </Show>
                  </div>
                </Show>
              </div>

              {/* Footer */}
              <div class="flex items-center justify-center px-6 py-4 border-t border-gray-200 dark:border-gray-700">
                <button
                  type="button"
                  onClick={props.onClose}
                  class="px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
                >
                  Close
                </button>
              </div>
            </div>
          </div>
        </div>
      </Show>
    </Portal>
  );
};
