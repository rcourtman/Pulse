import { Component, createSignal, onMount, For, Show } from 'solid-js';
import { useWebSocket } from '@/App';
import { showSuccess, showError } from '@/utils/toast';
import { NodeModal } from './NodeModal';
import { SettingsAPI } from '@/api/settings';

type SettingsTab = 'pve' | 'pbs' | 'system' | 'diagnostics';

interface NodeConfig {
  id: string;
  type: 'pve' | 'pbs';
  name: string;
  host: string;
  user?: string;
  hasPassword: boolean;
  tokenName?: string;
  hasToken: boolean;
  fingerprint?: string;
  verifySSL: boolean;
  monitorVMs?: boolean;
  monitorContainers?: boolean;
  monitorStorage?: boolean;
  monitorBackups?: boolean;
  monitorDatastores?: boolean;
  monitorSyncJobs?: boolean;
  monitorVerifyJobs?: boolean;
  monitorPruneJobs?: boolean;
  monitorGarbageJobs?: boolean;
  status: 'connected' | 'disconnected' | 'error';
}

const Settings: Component = () => {
  const { state, connected } = useWebSocket();
  const [activeTab, setActiveTab] = createSignal<SettingsTab>('pve');
  const [hasUnsavedChanges, setHasUnsavedChanges] = createSignal(false);
  const [nodes, setNodes] = createSignal<NodeConfig[]>([]);
  const [showNodeModal, setShowNodeModal] = createSignal(false);
  const [editingNode, setEditingNode] = createSignal<NodeConfig | null>(null);
  
  // System settings
  const [pollingInterval, setPollingInterval] = createSignal(5);
  const [backendPort, setBackendPort] = createSignal(3000);
  const [frontendPort, setFrontendPort] = createSignal(7655);
  const [showRestartModal, setShowRestartModal] = createSignal(false);
  const [pendingPortChanges, setPendingPortChanges] = createSignal<{backend?: number, frontend?: number} | null>(null);

  const tabs: { id: SettingsTab; label: string; icon: string }[] = [
    { 
      id: 'pve', 
      label: 'PVE Nodes',
      icon: 'M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01'
    },
    { 
      id: 'pbs', 
      label: 'PBS Nodes',
      icon: 'M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4'
    },
    { 
      id: 'system', 
      label: 'System',
      icon: 'M12 4v16m4-11h4m-4 6h4M8 9H4m4 6H4'
    },
    { 
      id: 'diagnostics', 
      label: 'Diagnostics',
      icon: 'M3 3v18h18m-10-8l3-3 3 3 4-4'
    }
  ];

  // Load nodes and system settings on mount
  onMount(async () => {
    try {
      // Load nodes
      const nodesResponse = await fetch('/api/config/nodes');
      if (nodesResponse.ok) {
        const data = await nodesResponse.json();
        setNodes(data);
      }
      
      // Load system settings
      try {
        const response = await SettingsAPI.getSettings();
        const settings = response.current;
        setPollingInterval((settings.monitoring.pollingInterval || 5000) / 1000);
        setBackendPort(settings.server.backend.port);
        setFrontendPort(settings.server.frontend.port);
      } catch (error) {
        console.error('Failed to load settings:', error);
      }
    } catch (error) {
      console.error('Failed to load configuration:', error);
    }
  });

  const validatePort = (port: number): string | null => {
    if (isNaN(port) || port < 1 || port > 65535) {
      return 'Port must be between 1 and 65535';
    }
    return null;
  };

  const saveSettings = async () => {
    try {
      if (activeTab() === 'system') {
        // Check if ports have changed
        const settingsResp = await SettingsAPI.getSettings();
        const currentSettings = settingsResp.current;
        const currentBackendPort = currentSettings.server.backend.port;
        const currentFrontendPort = currentSettings.server.frontend.port;
        const portChanged = 
          backendPort() !== currentBackendPort ||
          frontendPort() !== currentFrontendPort;
        
        if (portChanged) {
          // Validate ports
          const backendError = validatePort(backendPort());
          const frontendError = validatePort(frontendPort());
          
          if (backendError || frontendError) {
            showError(backendError || frontendError || 'Invalid port configuration');
            return;
          }
          
          if (backendPort() === frontendPort()) {
            showError('Backend and frontend ports must be different');
            return;
          }
          
          // Show restart confirmation modal
          setPendingPortChanges({
            backend: backendPort(),
            frontend: frontendPort()
          });
          setShowRestartModal(true);
          return;
        }
        
        // Save other system settings without restart
        const response = await fetch('/api/settings/update', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            monitoring: {
              pollingInterval: pollingInterval() * 1000
            }
          })
        });
        
        if (!response.ok) {
          throw new Error('Failed to save system settings');
        }
      }
      
      showSuccess('Settings saved successfully');
      setHasUnsavedChanges(false);
    } catch (error) {
      showError('Failed to save settings');
    }
  };
  
  const applyPortChanges = async () => {
    try {
      const response = await fetch('/api/settings/update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          server: {
            backend: { port: pendingPortChanges()?.backend },
            frontend: { port: pendingPortChanges()?.frontend }
          },
          monitoring: {
            pollingInterval: pollingInterval() * 1000
          }
        })
      });
      
      if (!response.ok) {
        throw new Error('Failed to update settings');
      }
      
      showSuccess('Settings saved. The application will restart...');
      setHasUnsavedChanges(false);
      setShowRestartModal(false);
      
      // The backend will handle the restart
      setTimeout(() => {
        window.location.href = `${window.location.protocol}//${window.location.hostname}:${pendingPortChanges()?.frontend}`;
      }, 3000);
    } catch (error) {
      showError('Failed to apply port changes');
      setShowRestartModal(false);
    }
  };

  const deleteNode = async (nodeId: string) => {
    if (!confirm('Are you sure you want to delete this node?')) return;
    
    try {
      const response = await fetch(`/api/config/nodes/${nodeId}`, {
        method: 'DELETE'
      });
      
      if (response.ok) {
        setNodes(nodes().filter(n => n.id !== nodeId));
        showSuccess('Node deleted successfully');
      } else {
        throw new Error('Failed to delete node');
      }
    } catch (error) {
      showError('Failed to delete node');
    }
  };

  const testNodeConnection = async (nodeId: string) => {
    try {
      const response = await fetch(`/api/config/nodes/${nodeId}/test`, {
        method: 'POST'
      });
      
      if (response.ok) {
        const result = await response.json();
        showSuccess(`Connection successful (${result.latency}ms)`);
      } else {
        throw new Error('Connection failed');
      }
    } catch (error) {
      showError('Connection test failed');
    }
  };

  return (
    <>
      {/* Restart Confirmation Modal */}
      <Show when={showRestartModal()}>
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
          <div class="bg-white dark:bg-gray-800 rounded-lg max-w-md w-full p-6">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">
              Confirm Port Changes
            </h3>
            
            <div class="space-y-4">
              <p class="text-sm text-gray-600 dark:text-gray-400">
                Changing ports will require restarting the application. You will be automatically redirected to the new port.
              </p>
              
              <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-3 space-y-2">
                <div class="text-sm">
                  <span class="text-gray-600 dark:text-gray-400">Backend Port:</span>
                  <span class="ml-2 font-medium">{pendingPortChanges()?.backend}</span>
                </div>
                <div class="text-sm">
                  <span class="text-gray-600 dark:text-gray-400">Frontend Port:</span>
                  <span class="ml-2 font-medium">{pendingPortChanges()?.frontend}</span>
                </div>
              </div>
              
              <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-3">
                <p class="text-xs text-amber-800 dark:text-amber-200">
                  <strong>Warning:</strong> Make sure these ports are not already in use by other applications.
                </p>
              </div>
            </div>
            
            <div class="flex justify-end gap-3 mt-6">
              <button
                onClick={() => {
                  setShowRestartModal(false);
                  setPendingPortChanges(null);
                }}
                class="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200"
              >
                Cancel
              </button>
              <button
                onClick={applyPortChanges}
                class="px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700"
              >
                Apply Changes & Restart
              </button>
            </div>
          </div>
        </div>
      </Show>

      <div class="space-y-4">
      {/* Header with better styling */}
      <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4">
        <div>
          <h1 class="text-xl font-semibold text-gray-800 dark:text-gray-200">Configuration Settings</h1>
          <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
            Manage Proxmox nodes and system configuration
          </p>
        </div>
      </div>
      
      {/* Save notification bar - only show when there are unsaved changes */}
      <Show when={hasUnsavedChanges() && (activeTab() === 'pve' || activeTab() === 'pbs' || activeTab() === 'system')}>
        <div class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-3 sm:p-4">
          <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
            <div class="flex items-center gap-2 text-yellow-800 dark:text-yellow-200">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="12" y1="8" x2="12" y2="12"></line>
                <line x1="12" y1="16" x2="12.01" y2="16"></line>
              </svg>
              <span class="text-sm font-medium">You have unsaved changes</span>
            </div>
            <div class="flex gap-2 w-full sm:w-auto">
              <button 
                class="flex-1 sm:flex-initial px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                onClick={saveSettings}
              >
                Save Changes
              </button>
              <button 
                class="flex-1 sm:flex-initial px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
                onClick={() => {
                  window.location.reload();
                }}
              >
                Discard
              </button>
            </div>
          </div>
        </div>
      </Show>
      
      {/* Tab Navigation - modern style */}
      <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-sm">
        <div class="p-1">
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5 w-full overflow-x-auto">
            <For each={tabs}>
              {(tab) => (
                <button
                  class={`flex-1 px-3 py-2 text-xs sm:text-sm font-medium rounded-md transition-all whitespace-nowrap ${
                    activeTab() === tab.id
                      ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
                  onClick={() => setActiveTab(tab.id)}
                >
                  {tab.label}
                </button>
              )}
            </For>
          </div>
        </div>
        <div class="border-t border-gray-200 dark:border-gray-700"></div>
        
        {/* Tab Content */}
        <div class="p-3 sm:p-6">
          {/* PVE Nodes Tab */}
          <Show when={activeTab() === 'pve'}>
            <div class="space-y-4">
              <div class="flex items-center justify-between mb-4">
                <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Proxmox VE Nodes</h3>
                <button 
                  onClick={() => {
                    setEditingNode(null);
                    setShowNodeModal(true);
                  }}
                  class="px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors flex items-center gap-2"
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <line x1="12" y1="5" x2="12" y2="19"></line>
                    <line x1="5" y1="12" x2="19" y2="12"></line>
                  </svg>
                  Add PVE Node
                </button>
              </div>
              
              <div class="grid gap-4">
                <For each={nodes().filter(n => n.type === 'pve')}>
                  {(node) => (
                    <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4 border border-gray-200 dark:border-gray-600">
                      <div class="flex items-start justify-between">
                        <div class="flex items-start gap-3">
                          <div class={`w-3 h-3 rounded-full mt-1.5 ${
                            node.status === 'connected' ? 'bg-green-500' : 
                            node.status === 'error' ? 'bg-red-500' : 'bg-gray-400'
                          }`}></div>
                          <div>
                            <h4 class="font-medium text-gray-900 dark:text-gray-100">{node.name}</h4>
                            <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">{node.host}</p>
                            <div class="flex flex-wrap gap-2 mt-2">
                              <span class="text-xs px-2 py-1 bg-gray-200 dark:bg-gray-600 rounded">
                                {node.user ? `User: ${node.user}` : `Token: ${node.tokenName}`}
                              </span>
                              {node.monitorVMs && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">VMs</span>}
                              {node.monitorContainers && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Containers</span>}
                              {node.monitorStorage && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Storage</span>}
                              {node.monitorBackups && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Backups</span>}
                            </div>
                          </div>
                        </div>
                        <div class="flex items-center gap-2">
                          <button
                            onClick={() => testNodeConnection(node.id)}
                            class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                            title="Test connection"
                          >
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                              <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
                            </svg>
                          </button>
                          <button
                            onClick={() => {
                              setEditingNode(node);
                              setShowNodeModal(true);
                            }}
                            class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                          >
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                              <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"></path>
                              <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                            </svg>
                          </button>
                          <button
                            onClick={() => deleteNode(node.id)}
                            class="p-2 text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300"
                          >
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                              <polyline points="3 6 5 6 21 6"></polyline>
                              <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"></path>
                            </svg>
                          </button>
                        </div>
                      </div>
                    </div>
                  )}
                </For>
                
                {nodes().filter(n => n.type === 'pve').length === 0 && (
                  <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                    <p>No PVE nodes configured</p>
                    <p class="text-sm mt-1">Add a node to start monitoring</p>
                  </div>
                )}
              </div>
            </div>
          </Show>
          
          {/* PBS Nodes Tab */}
          <Show when={activeTab() === 'pbs'}>
            <div class="space-y-4">
              <div class="flex items-center justify-between mb-4">
                <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Proxmox Backup Server Nodes</h3>
                <button 
                  onClick={() => {
                    setEditingNode(null);
                    setShowNodeModal(true);
                  }}
                  class="px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors flex items-center gap-2"
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <line x1="12" y1="5" x2="12" y2="19"></line>
                    <line x1="5" y1="12" x2="19" y2="12"></line>
                  </svg>
                  Add PBS Node
                </button>
              </div>
              
              <div class="grid gap-4">
                <For each={nodes().filter(n => n.type === 'pbs')}>
                  {(node) => (
                    <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4 border border-gray-200 dark:border-gray-600">
                      <div class="flex items-start justify-between">
                        <div class="flex items-start gap-3">
                          <div class={`w-3 h-3 rounded-full mt-1.5 ${
                            node.status === 'connected' ? 'bg-green-500' : 
                            node.status === 'error' ? 'bg-red-500' : 'bg-gray-400'
                          }`}></div>
                          <div>
                            <h4 class="font-medium text-gray-900 dark:text-gray-100">{node.name}</h4>
                            <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">{node.host}</p>
                            <div class="flex flex-wrap gap-2 mt-2">
                              <span class="text-xs px-2 py-1 bg-gray-200 dark:bg-gray-600 rounded">
                                {node.user ? `User: ${node.user}` : `Token: ${node.tokenName}`}
                              </span>
                              {node.monitorDatastores && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Datastores</span>}
                              {node.monitorSyncJobs && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Sync Jobs</span>}
                              {node.monitorVerifyJobs && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Verify Jobs</span>}
                              {node.monitorPruneJobs && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Prune Jobs</span>}
                            </div>
                          </div>
                        </div>
                        <div class="flex items-center gap-2">
                          <button
                            onClick={() => testNodeConnection(node.id)}
                            class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                            title="Test connection"
                          >
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                              <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
                            </svg>
                          </button>
                          <button
                            onClick={() => {
                              setEditingNode(node);
                              setShowNodeModal(true);
                            }}
                            class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                          >
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                              <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"></path>
                              <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                            </svg>
                          </button>
                          <button
                            onClick={() => deleteNode(node.id)}
                            class="p-2 text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300"
                          >
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                              <polyline points="3 6 5 6 21 6"></polyline>
                              <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"></path>
                            </svg>
                          </button>
                        </div>
                      </div>
                    </div>
                  )}
                </For>
                
                {nodes().filter(n => n.type === 'pbs').length === 0 && (
                  <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                    <p>No PBS nodes configured</p>
                    <p class="text-sm mt-1">Add a node to start monitoring</p>
                  </div>
                )}
              </div>
            </div>
          </Show>
          
          {/* System Settings Tab */}
          <Show when={activeTab() === 'system'}>
            <div class="space-y-6">
              <div>
                <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">System Configuration</h3>
                
                <div class="space-y-4">
                  {/* Polling Settings */}
                  <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4">
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4 flex items-center gap-2">
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"></path>
                      </svg>
                      Performance Settings
                    </h4>
                    
                    <div class="space-y-4">
                      {/* Port Configuration */}
                      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div>
                          <label class="text-sm font-medium text-gray-900 dark:text-gray-100">Backend Port</label>
                          <p class="text-xs text-gray-600 dark:text-gray-400 mb-2">API server port</p>
                          <input
                            type="number"
                            value={backendPort()}
                            onInput={(e) => {
                              const val = parseInt(e.currentTarget.value);
                              if (!isNaN(val) && val >= 1 && val <= 65535) {
                                setBackendPort(val);
                                setHasUnsavedChanges(true);
                              }
                            }}
                            min="1"
                            max="65535"
                            class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-green-500 focus:border-green-500 dark:bg-gray-700 dark:text-white"
                            placeholder="3000"
                          />
                        </div>
                        
                        <div>
                          <label class="text-sm font-medium text-gray-900 dark:text-gray-100">Frontend Port</label>
                          <p class="text-xs text-gray-600 dark:text-gray-400 mb-2">Web UI port</p>
                          <input
                            type="number"
                            value={frontendPort()}
                            onInput={(e) => {
                              const val = parseInt(e.currentTarget.value);
                              if (!isNaN(val)) {
                                setFrontendPort(val);
                                setHasUnsavedChanges(true);
                              }
                            }}
                            min="1"
                            max="65535"
                            class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600 
                                   focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                          />
                        </div>
                      </div>
                      
                      <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-3">
                        <div class="flex items-start gap-2">
                          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="text-amber-600 dark:text-amber-400 mt-0.5">
                            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
                            <line x1="12" y1="9" x2="12" y2="13"></line>
                            <line x1="12" y1="17" x2="12.01" y2="17"></line>
                          </svg>
                          <div class="text-xs text-amber-800 dark:text-amber-200">
                            <p class="font-medium mb-1">Changing ports requires restart</p>
                            <p>The application will restart automatically when you save port changes. Make sure the new ports are not already in use.</p>
                          </div>
                        </div>
                      </div>
                      
                      <div class="border-t dark:border-gray-600 pt-4">
                        <div class="flex items-center justify-between">
                          <div>
                            <label class="text-sm font-medium text-gray-900 dark:text-gray-100">Polling Interval</label>
                            <p class="text-xs text-gray-600 dark:text-gray-400">
                              How often to fetch data from servers
                            </p>
                        </div>
                        <select
                          value={pollingInterval()}
                          onChange={(e) => {
                            setPollingInterval(parseInt(e.currentTarget.value));
                            setHasUnsavedChanges(true);
                          }}
                          class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800"
                        >
                          <option value="3">3 seconds</option>
                          <option value="5">5 seconds</option>
                          <option value="10">10 seconds</option>
                          <option value="30">30 seconds</option>
                          <option value="60">1 minute</option>
                        </select>
                      </div>
                      </div>
                      
                    </div>
                  </div>
                  
                </div>
              </div>
            </div>
          </Show>
          
          {/* Diagnostics Tab */}
          <Show when={activeTab() === 'diagnostics'}>
            <div class="space-y-6">
              <div>
                <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">System Diagnostics</h3>
                
                <div class="space-y-4">
                  {/* System Information */}
                  <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4">
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4">System Information</h4>
                    <div class="space-y-2 text-sm">
                      <div class="flex justify-between">
                        <span class="text-gray-600 dark:text-gray-400">Version:</span>
                        <span class="font-medium">2.0.0</span>
                      </div>
                      <div class="flex justify-between">
                        <span class="text-gray-600 dark:text-gray-400">Backend:</span>
                        <span class="font-medium">Go 1.21</span>
                      </div>
                      <div class="flex justify-between">
                        <span class="text-gray-600 dark:text-gray-400">Frontend:</span>
                        <span class="font-medium">SolidJS + TypeScript</span>
                      </div>
                      <div class="flex justify-between">
                        <span class="text-gray-600 dark:text-gray-400">WebSocket Status:</span>
                        <span class={`font-medium ${connected() ? 'text-green-600' : 'text-red-600'}`}>
                          {connected() ? 'Connected' : 'Disconnected'}
                        </span>
                      </div>
                    </div>
                  </div>
                  
                  {/* Connection Status */}
                  <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4">
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4">Connection Status</h4>
                    <div class="space-y-2 text-sm">
                      <div class="flex justify-between">
                        <span class="text-gray-600 dark:text-gray-400">PVE Nodes:</span>
                        <span class="font-medium">{nodes().filter(n => n.type === 'pve').length}</span>
                      </div>
                      <div class="flex justify-between">
                        <span class="text-gray-600 dark:text-gray-400">PBS Nodes:</span>
                        <span class="font-medium">{nodes().filter(n => n.type === 'pbs').length}</span>
                      </div>
                      <div class="flex justify-between">
                        <span class="text-gray-600 dark:text-gray-400">Total VMs:</span>
                        <span class="font-medium">{state.vms?.length || 0}</span>
                      </div>
                      <div class="flex justify-between">
                        <span class="text-gray-600 dark:text-gray-400">Total Containers:</span>
                        <span class="font-medium">{state.containers?.length || 0}</span>
                      </div>
                    </div>
                  </div>
                  
                  {/* Export Diagnostics */}
                  <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4">
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4">Export Diagnostics</h4>
                    <p class="text-xs text-gray-600 dark:text-gray-400 mb-4">
                      Export system diagnostics data for troubleshooting
                    </p>
                    <button
                      onClick={() => {
                        const diagnostics = {
                          timestamp: new Date().toISOString(),
                          version: '2.0.0',
                          environment: {
                            userAgent: navigator.userAgent,
                            platform: navigator.platform,
                            language: navigator.language,
                            screenResolution: `${window.screen.width}x${window.screen.height}`,
                            windowSize: `${window.innerWidth}x${window.innerHeight}`,
                            timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone
                          },
                          websocket: {
                            connected: connected(),
                            url: window.location.hostname === 'localhost' 
                              ? 'ws://localhost:3000/ws' 
                              : `ws://${window.location.hostname}:3000/ws`
                          },
                          nodes: nodes(),
                          state: {
                            nodesCount: state.nodes?.length || 0,
                            vmsCount: state.vms?.length || 0,
                            containersCount: state.containers?.length || 0,
                            storageCount: state.storage?.length || 0
                          },
                          settings: {
                            pollingInterval: pollingInterval()
                          }
                        };
                        
                        const blob = new Blob([JSON.stringify(diagnostics, null, 2)], { type: 'application/json' });
                        const url = URL.createObjectURL(blob);
                        const a = document.createElement('a');
                        a.href = url;
                        a.download = `pulse-diagnostics-${new Date().toISOString().split('T')[0]}.json`;
                        document.body.appendChild(a);
                        a.click();
                        document.body.removeChild(a);
                        URL.revokeObjectURL(url);
                      }}
                      class="px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                    >
                      Export Diagnostics
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </Show>
        </div>
      </div>
      
      {/* Node Modal */}
      <NodeModal
        isOpen={showNodeModal()}
        onClose={() => {
          setShowNodeModal(false);
          setEditingNode(null);
        }}
        nodeType={activeTab() === 'pve' ? 'pve' : 'pbs'}
        editingNode={editingNode()}
        onSave={async (nodeData) => {
          try {
            if (editingNode()) {
              // Update existing node
              const response = await fetch(`/api/config/nodes/${editingNode()!.id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(nodeData)
              });
              
              if (response.ok) {
                // Update local state
                setNodes(nodes().map(n => 
                  n.id === editingNode()!.id 
                    ? { ...n, ...nodeData, hasPassword: !!nodeData.password, hasToken: !!nodeData.tokenValue }
                    : n
                ));
                showSuccess('Node updated successfully');
              } else {
                throw new Error('Failed to update node');
              }
            } else {
              // Add new node
              const response = await fetch('/api/config/nodes', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(nodeData)
              });
              
              if (response.ok) {
                // Reload nodes to get the new ID
                const nodesResponse = await fetch('/api/config/nodes');
                if (nodesResponse.ok) {
                  const updatedNodes = await nodesResponse.json();
                  setNodes(updatedNodes);
                }
                showSuccess('Node added successfully');
              } else {
                throw new Error('Failed to add node');
              }
            }
            
            setShowNodeModal(false);
            setEditingNode(null);
          } catch (error) {
            showError(error instanceof Error ? error.message : 'Operation failed');
          }
        }}
      />
    </div>
    </>
  );
};

export default Settings;