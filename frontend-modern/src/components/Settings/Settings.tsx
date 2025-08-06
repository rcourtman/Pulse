import { Component, createSignal, onMount, For, Show, createEffect } from 'solid-js';
import { useWebSocket } from '@/App';
import { showSuccess, showError } from '@/utils/toast';
import { NodeModal } from './NodeModal';
import { SettingsAPI } from '@/api/settings';
import { NodesAPI } from '@/api/nodes';
import { UpdatesAPI } from '@/api/updates';
import type { NodeConfig } from '@/types/nodes';
import type { UpdateInfo, UpdateStatus, VersionInfo } from '@/api/updates';

type SettingsTab = 'pve' | 'pbs' | 'system' | 'diagnostics';

// Node with UI-specific fields
type NodeConfigWithStatus = NodeConfig & {
  hasPassword?: boolean;
  hasToken?: boolean;
  status: 'connected' | 'disconnected' | 'error';
};

const Settings: Component = () => {
  const { state, connected, updateProgress } = useWebSocket();
  const [activeTab, setActiveTab] = createSignal<SettingsTab>('pve');
  const [hasUnsavedChanges, setHasUnsavedChanges] = createSignal(false);
  const [nodes, setNodes] = createSignal<NodeConfigWithStatus[]>([]);
  const [showNodeModal, setShowNodeModal] = createSignal(false);
  const [editingNode, setEditingNode] = createSignal<NodeConfigWithStatus | null>(null);
  
  // System settings
  const [pollingInterval, setPollingInterval] = createSignal(5);
  const [backendPort, setBackendPort] = createSignal(3000);
  const [frontendPort, setFrontendPort] = createSignal(7655);
  const [allowedOrigins, setAllowedOrigins] = createSignal('*');
  const [connectionTimeout, setConnectionTimeout] = createSignal(10);
  
  // Update settings
  const [versionInfo, setVersionInfo] = createSignal<VersionInfo | null>(null);
  const [updateInfo, setUpdateInfo] = createSignal<UpdateInfo | null>(null);
  const [updateStatus, setUpdateStatus] = createSignal<UpdateStatus | null>(null);
  const [checkingForUpdates, setCheckingForUpdates] = createSignal(false);
  const [updateChannel, setUpdateChannel] = createSignal<'stable' | 'rc'>('stable');
  const [autoUpdateEnabled, setAutoUpdateEnabled] = createSignal(false);
  const [autoUpdateCheckInterval, setAutoUpdateCheckInterval] = createSignal(24);
  const [autoUpdateTime, setAutoUpdateTime] = createSignal('03:00');
  
  // Diagnostics
  const [diagnosticsData, setDiagnosticsData] = createSignal<any>(null);
  const [runningDiagnostics, setRunningDiagnostics] = createSignal(false);

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

  // Update status from WebSocket events
  createEffect(() => {
    const progress = updateProgress();
    if (progress) {
      setUpdateStatus(progress);
    }
  });
  
  // Load nodes and system settings on mount
  onMount(async () => {
    try {
      // Load nodes
      const nodesList = await NodesAPI.getNodes();
      // Add status and other UI fields
      const nodesWithStatus = nodesList.map(node => ({
        ...node,
        // Use the hasPassword/hasToken from the API if available, otherwise check local fields
        hasPassword: node.hasPassword ?? !!node.password,
        hasToken: node.hasToken ?? !!node.tokenValue,
        status: node.status || 'disconnected' as const
      }));
      setNodes(nodesWithStatus);
      
      // Load system settings
      try {
        const systemResponse = await fetch('/api/config/system');
        if (systemResponse.ok) {
          const systemSettings = await systemResponse.json();
          setPollingInterval(systemSettings.pollingInterval || 5);
          setBackendPort(systemSettings.backendPort || 3000);
          setFrontendPort(systemSettings.frontendPort || 7655);
          setAllowedOrigins(systemSettings.allowedOrigins || '*');
          setConnectionTimeout(systemSettings.connectionTimeout || 10);
        } else {
          // Fallback to old endpoint
          const response = await SettingsAPI.getSettings();
          const settings = response.current;
          setPollingInterval((settings.monitoring.pollingInterval || 5000) / 1000);
        }
      } catch (error) {
        console.error('Failed to load settings:', error);
      }
      
      // Load version information
      try {
        const version = await UpdatesAPI.getVersion();
        setVersionInfo(version);
        if (version.channel) {
          setUpdateChannel(version.channel as 'stable' | 'rc');
        }
      } catch (error) {
        console.error('Failed to load version:', error);
      }
    } catch (error) {
      console.error('Failed to load configuration:', error);
    }
  });

  const saveSettings = async () => {
    try {
      if (activeTab() === 'system') {
        // Save system settings using typed API
        await SettingsAPI.updateSystemSettings({
          pollingInterval: pollingInterval(),
          backendPort: backendPort(),
          frontendPort: frontendPort(),
          allowedOrigins: allowedOrigins(),
          connectionTimeout: connectionTimeout(),
          updateChannel: updateChannel(),
          autoUpdateEnabled: autoUpdateEnabled(),
          autoUpdateCheckInterval: autoUpdateCheckInterval(),
          autoUpdateTime: autoUpdateTime()
        });
      }
      
      showSuccess('Settings saved successfully. Service restart may be required for port changes.');
      setHasUnsavedChanges(false);
      
      // Reload the page after a short delay to ensure the new settings are applied
      setTimeout(() => {
        window.location.reload();
      }, 3000);
    } catch (error) {
      showError(error instanceof Error ? error.message : 'Failed to save settings');
    }
  };

  const deleteNode = async (nodeId: string) => {
    if (!confirm('Are you sure you want to delete this node?')) return;
    
    try {
      await NodesAPI.deleteNode(nodeId);
      setNodes(nodes().filter(n => n.id !== nodeId));
      showSuccess('Node deleted successfully');
    } catch (error) {
      showError(error instanceof Error ? error.message : 'Failed to delete node');
    }
  };

  const testNodeConnection = async (nodeId: string) => {
    try {
      const node = nodes().find(n => n.id === nodeId);
      if (!node) {
        throw new Error('Node not found');
      }
      
      // Use the existing node test endpoint which uses stored credentials
      const result = await NodesAPI.testExistingNode(nodeId);
      if (result.status === 'success') {
        showSuccess(result.message || 'Connection successful');
      } else {
        throw new Error(result.message || 'Connection failed');
      }
    } catch (error) {
      showError(error instanceof Error ? error.message : 'Connection test failed');
    }
  };
  
  const checkForUpdates = async () => {
    setCheckingForUpdates(true);
    try {
      const info = await UpdatesAPI.checkForUpdates();
      setUpdateInfo(info);
      if (!info.available) {
        showSuccess('You are running the latest version');
      }
    } catch (error) {
      showError('Failed to check for updates');
      console.error('Update check error:', error);
    } finally {
      setCheckingForUpdates(false);
    }
  };
  
  const applyUpdate = async () => {
    const info = updateInfo();
    if (!info || !info.downloadUrl) {
      showError('No update available');
      return;
    }
    
    try {
      await UpdatesAPI.applyUpdate(info.downloadUrl);
      showSuccess('Update started. Pulse will restart automatically.');
      
      // Start polling for update status
      const pollStatus = setInterval(async () => {
        try {
          const status = await UpdatesAPI.getUpdateStatus();
          setUpdateStatus(status);
          
          if (status.status === 'completed' || status.status === 'error') {
            clearInterval(pollStatus);
            if (status.status === 'error') {
              showError(status.error || 'Update failed');
            }
          }
        } catch (error) {
          // Service might be restarting
          console.log('Status check failed, service may be restarting');
        }
      }, 1000);
    } catch (error) {
      showError('Failed to start update');
      console.error('Update error:', error);
    }
  };

  return (
    <>
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
                            (() => {
                              // Find the corresponding node in the WebSocket state
                              const stateNode = state.nodes.find(n => n.instance === node.name);
                              // Check if the node has an error status or is offline
                              if (stateNode?.connectionHealth === 'error' || stateNode?.status === 'offline') {
                                return 'bg-red-500';
                              }
                              // Check if we have a healthy connection
                              if (stateNode && stateNode.status === 'online') {
                                return 'bg-green-500';
                              }
                              // Default to gray if no state data
                              return 'bg-gray-400';
                            })()
                          }`}></div>
                          <div>
                            <h4 class="font-medium text-gray-900 dark:text-gray-100">{node.name}</h4>
                            <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">{node.host}</p>
                            <div class="flex flex-wrap gap-2 mt-2">
                              <span class="text-xs px-2 py-1 bg-gray-200 dark:bg-gray-600 rounded">
                                {node.user ? `User: ${node.user}` : `Token: ${node.tokenName}`}
                              </span>
                              {node.type === 'pve' && 'monitorVMs' in node && node.monitorVMs && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">VMs</span>}
                              {node.type === 'pve' && 'monitorContainers' in node && node.monitorContainers && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Containers</span>}
                              {node.type === 'pve' && 'monitorStorage' in node && node.monitorStorage && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Storage</span>}
                              {node.type === 'pve' && 'monitorBackups' in node && node.monitorBackups && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Backups</span>}
                            </div>
                            <Show when={node.type === 'pve' && 'isCluster' in node && node.isCluster}>
                              <div class="mt-2 text-xs text-gray-600 dark:text-gray-400">
                                <div class="font-medium mb-1">Cluster: {'clusterName' in node ? node.clusterName : 'Unknown'}</div>
                                <div class="flex flex-wrap gap-1">
                                  <For each={'clusterEndpoints' in node ? node.clusterEndpoints : []}>
                                    {(endpoint) => (
                                      <span class="px-2 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">
                                        {endpoint.NodeName} ({endpoint.IP})
                                      </span>
                                    )}
                                  </For>
                                </div>
                                <p class="mt-1 text-xs text-gray-500 dark:text-gray-500 italic">
                                  Pulse will automatically failover between cluster nodes
                                </p>
                              </div>
                            </Show>
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
                            (() => {
                              // Find the corresponding PBS instance in the WebSocket state
                              const statePBS = state.pbs.find(p => p.name === node.name);
                              // Check if the PBS has an error status or is offline
                              if (statePBS?.connectionHealth === 'error' || statePBS?.status === 'offline') {
                                return 'bg-red-500';
                              }
                              // Check if we have a healthy connection
                              if (statePBS && statePBS.status === 'online') {
                                return 'bg-green-500';
                              }
                              // Default to gray if no state data
                              return 'bg-gray-400';
                            })()
                          }`}></div>
                          <div>
                            <h4 class="font-medium text-gray-900 dark:text-gray-100">{node.name}</h4>
                            <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">{node.host}</p>
                            <div class="flex flex-wrap gap-2 mt-2">
                              <span class="text-xs px-2 py-1 bg-gray-200 dark:bg-gray-600 rounded">
                                {node.user ? `User: ${node.user}` : `Token: ${node.tokenName}`}
                              </span>
                              {node.type === 'pbs' && 'monitorDatastores' in node && node.monitorDatastores && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Datastores</span>}
                              {node.type === 'pbs' && 'monitorSyncJobs' in node && node.monitorSyncJobs && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Sync Jobs</span>}
                              {node.type === 'pbs' && 'monitorVerifyJobs' in node && node.monitorVerifyJobs && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Verify Jobs</span>}
                              {node.type === 'pbs' && 'monitorPruneJobs' in node && node.monitorPruneJobs && <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">Prune Jobs</span>}
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
                  {/* Performance Settings */}
                  <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4">
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4 flex items-center gap-2">
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"></path>
                      </svg>
                      Performance Settings
                    </h4>
                    
                    <div class="space-y-4">
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
                      
                      <div class="flex items-center justify-between">
                        <div>
                          <label class="text-sm font-medium text-gray-900 dark:text-gray-100">Connection Timeout</label>
                          <p class="text-xs text-gray-600 dark:text-gray-400">
                            Max wait time for node responses
                          </p>
                        </div>
                        <select
                          value={connectionTimeout()}
                          onChange={(e) => {
                            setConnectionTimeout(parseInt(e.currentTarget.value));
                            setHasUnsavedChanges(true);
                          }}
                          class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800"
                        >
                          <option value="5">5 seconds</option>
                          <option value="10">10 seconds</option>
                          <option value="20">20 seconds</option>
                          <option value="30">30 seconds</option>
                        </select>
                      </div>
                    </div>
                  </div>
                  
                  {/* Network Settings */}
                  <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4">
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4 flex items-center gap-2">
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <circle cx="12" cy="12" r="10"></circle>
                        <path d="M2 12h20M12 2a15.3 15.3 0 014 10 15.3 15.3 0 01-4 10 15.3 15.3 0 01-4-10 15.3 15.3 0 014-10z"></path>
                      </svg>
                      Network Settings
                    </h4>
                    
                    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div>
                        <label class="text-sm font-medium text-gray-900 dark:text-gray-100">Backend Port</label>
                        <p class="text-xs text-gray-600 dark:text-gray-400 mb-2">API server port</p>
                        <input
                          type="number"
                          value={backendPort()}
                          onChange={(e) => {
                            setBackendPort(parseInt(e.currentTarget.value));
                            setHasUnsavedChanges(true);
                          }}
                          min="1"
                          max="65535"
                          class="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800"
                        />
                      </div>
                      
                      <div>
                        <label class="text-sm font-medium text-gray-900 dark:text-gray-100">Frontend Port</label>
                        <p class="text-xs text-gray-600 dark:text-gray-400 mb-2">Web UI port</p>
                        <input
                          type="number"
                          value={frontendPort()}
                          onChange={(e) => {
                            setFrontendPort(parseInt(e.currentTarget.value));
                            setHasUnsavedChanges(true);
                          }}
                          min="1"
                          max="65535"
                          class="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800"
                        />
                      </div>
                    </div>
                    
                    <div class="mt-4">
                      <label class="text-sm font-medium text-gray-900 dark:text-gray-100">CORS Allowed Origins</label>
                      <p class="text-xs text-gray-600 dark:text-gray-400 mb-2">For reverse proxy setups (* = allow all)</p>
                      <input
                        type="text"
                        value={allowedOrigins()}
                        onChange={(e) => {
                          setAllowedOrigins(e.currentTarget.value);
                          setHasUnsavedChanges(true);
                        }}
                        placeholder="* or https://example.com"
                        class="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800"
                      />
                    </div>
                    
                    <div class="mt-3 p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg">
                      <p class="text-xs text-amber-800 dark:text-amber-200">
                        <strong>Note:</strong> Port changes require a service restart
                      </p>
                    </div>
                  </div>
                  
                  {/* Update Settings */}
                  <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4">
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4 flex items-center gap-2">
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="23 4 23 10 17 10"></polyline>
                        <polyline points="1 20 1 14 7 14"></polyline>
                        <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"></path>
                      </svg>
                      Updates
                    </h4>
                    
                    <div class="space-y-4">
                      {/* Version Info */}
                      <div class="flex items-center justify-between">
                        <div>
                          <label class="text-sm font-medium text-gray-900 dark:text-gray-100">Current Version</label>
                          <p class="text-xs text-gray-600 dark:text-gray-400">
                            {versionInfo()?.version || 'Loading...'} 
                            {versionInfo()?.isDevelopment && ' (Development)'}
                            {versionInfo()?.isDocker && ' - Docker'}
                          </p>
                        </div>
                        <button
                          onClick={checkForUpdates}
                          disabled={checkingForUpdates() || versionInfo()?.isDocker}
                          class={`px-4 py-2 text-sm rounded-lg transition-colors flex items-center gap-2 ${
                            versionInfo()?.isDocker
                              ? 'bg-gray-100 dark:bg-gray-700 text-gray-400 dark:text-gray-500 cursor-not-allowed'
                              : 'bg-blue-600 text-white hover:bg-blue-700'
                          }`}
                        >
                          {checkingForUpdates() ? (
                            <>
                              <div class="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full"></div>
                              Checking...
                            </>
                          ) : (
                            <>Check for Updates</>
                          )}
                        </button>
                      </div>
                      
                      {/* Docker Message */}
                      <Show when={versionInfo()?.isDocker}>
                        <div class="p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
                          <p class="text-xs text-blue-800 dark:text-blue-200">
                            <strong>Docker Installation:</strong> Updates are managed through Docker. Pull the latest image to update.
                          </p>
                        </div>
                      </Show>
                      
                      {/* Update Available */}
                      <Show when={updateInfo()?.available && !versionInfo()?.isDocker}>
                        <div class="p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg">
                          <div class="flex items-start justify-between mb-2">
                            <div>
                              <p class="text-sm font-medium text-green-800 dark:text-green-200">
                                Update Available: {updateInfo()?.latestVersion}
                              </p>
                              <p class="text-xs text-green-700 dark:text-green-300 mt-1">
                                Released: {updateInfo()?.releaseDate ? new Date(updateInfo()!.releaseDate).toLocaleDateString() : 'Unknown'}
                              </p>
                            </div>
                            <Show when={!updateStatus() || updateStatus()?.status === 'idle'}>
                              <button
                                onClick={applyUpdate}
                                class="px-3 py-1.5 text-xs bg-green-600 text-white rounded-lg hover:bg-green-700 transition-colors"
                              >
                                Apply Update
                              </button>
                            </Show>
                          </div>
                          <Show when={updateInfo()?.releaseNotes}>
                            <details class="mt-2">
                              <summary class="text-xs text-green-700 dark:text-green-300 cursor-pointer">Release Notes</summary>
                              <pre class="mt-2 text-xs text-green-600 dark:text-green-400 whitespace-pre-wrap font-mono bg-green-100 dark:bg-green-900/30 p-2 rounded">
                                {updateInfo()?.releaseNotes}
                              </pre>
                            </details>
                          </Show>
                        </div>
                      </Show>
                      
                      {/* Update Progress */}
                      <Show when={updateStatus() && updateStatus()?.status !== 'idle'}>
                        <div class="p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
                          <div class="mb-2">
                            <p class="text-sm font-medium text-blue-800 dark:text-blue-200">
                              {updateStatus()?.message || 'Processing update...'}
                            </p>
                          </div>
                          <Show when={updateStatus()?.progress}>
                            <div class="w-full bg-blue-200 dark:bg-blue-800 rounded-full h-2">
                              <div
                                class="bg-blue-600 h-2 rounded-full transition-all duration-300"
                                style={`width: ${updateStatus()?.progress}%`}
                              ></div>
                            </div>
                          </Show>
                        </div>
                      </Show>
                      
                      {/* Update Settings */}
                      <div class="border-t border-gray-200 dark:border-gray-600 pt-4 space-y-4">
                        <div class="flex items-center justify-between">
                          <div>
                            <label class="text-sm font-medium text-gray-900 dark:text-gray-100">Update Channel</label>
                            <p class="text-xs text-gray-600 dark:text-gray-400">
                              Choose between stable and release candidate versions
                            </p>
                          </div>
                          <select
                            value={updateChannel()}
                            onChange={(e) => {
                              setUpdateChannel(e.currentTarget.value as 'stable' | 'rc');
                              setHasUnsavedChanges(true);
                            }}
                            disabled={versionInfo()?.isDocker}
                            class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 disabled:opacity-50"
                          >
                            <option value="stable">Stable</option>
                            <option value="rc">Release Candidate</option>
                          </select>
                        </div>
                        
                        <div class="flex items-center justify-between">
                          <div>
                            <label class="text-sm font-medium text-gray-900 dark:text-gray-100">Auto-Update</label>
                            <p class="text-xs text-gray-600 dark:text-gray-400">
                              Automatically install updates when available
                            </p>
                          </div>
                          <label class="relative inline-flex items-center cursor-pointer">
                            <input
                              type="checkbox"
                              checked={autoUpdateEnabled()}
                              onChange={(e) => {
                                setAutoUpdateEnabled(e.currentTarget.checked);
                                setHasUnsavedChanges(true);
                              }}
                              disabled={versionInfo()?.isDocker}
                              class="sr-only peer"
                            />
                            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600 peer-disabled:opacity-50"></div>
                          </label>
                        </div>
                        
                        <Show when={autoUpdateEnabled()}>
                          <div class="pl-4 space-y-4 border-l-2 border-gray-200 dark:border-gray-600">
                            <div class="flex items-center justify-between">
                              <div>
                                <label class="text-sm font-medium text-gray-900 dark:text-gray-100">Check Interval</label>
                                <p class="text-xs text-gray-600 dark:text-gray-400">
                                  How often to check for updates
                                </p>
                              </div>
                              <select
                                value={autoUpdateCheckInterval()}
                                onChange={(e) => {
                                  setAutoUpdateCheckInterval(parseInt(e.currentTarget.value));
                                  setHasUnsavedChanges(true);
                                }}
                                class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800"
                              >
                                <option value="6">Every 6 hours</option>
                                <option value="12">Every 12 hours</option>
                                <option value="24">Daily</option>
                                <option value="168">Weekly</option>
                              </select>
                            </div>
                            
                            <div class="flex items-center justify-between">
                              <div>
                                <label class="text-sm font-medium text-gray-900 dark:text-gray-100">Update Time</label>
                                <p class="text-xs text-gray-600 dark:text-gray-400">
                                  Preferred time for automatic updates
                                </p>
                              </div>
                              <input
                                type="time"
                                value={autoUpdateTime()}
                                onChange={(e) => {
                                  setAutoUpdateTime(e.currentTarget.value);
                                  setHasUnsavedChanges(true);
                                }}
                                class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800"
                              />
                            </div>
                          </div>
                        </Show>
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
                  {/* Live Connection Diagnostics */}
                  <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4">
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4">Connection Diagnostics</h4>
                    <p class="text-xs text-gray-600 dark:text-gray-400 mb-4">
                      Test all configured node connections and view detailed status
                    </p>
                    <button
                      onClick={async () => {
                        setRunningDiagnostics(true);
                        try {
                          const response = await fetch('/api/diagnostics');
                          const diag = await response.json();
                          setDiagnosticsData(diag);
                        } catch (err) {
                          console.error('Failed to fetch diagnostics:', err);
                          showError('Failed to run diagnostics');
                        } finally {
                          setRunningDiagnostics(false);
                        }
                      }}
                      disabled={runningDiagnostics()}
                      class="px-4 py-2 text-sm bg-green-600 text-white rounded-lg hover:bg-green-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      {runningDiagnostics() ? 'Running...' : 'Run Diagnostics'}
                    </button>
                    
                    <Show when={diagnosticsData()}>
                      <div class="mt-4 space-y-3">
                        {/* System Info */}
                        <div class="bg-white dark:bg-gray-800 rounded-lg p-3">
                          <h5 class="text-sm font-semibold mb-2 text-gray-700 dark:text-gray-300">System</h5>
                          <div class="text-xs space-y-1 text-gray-600 dark:text-gray-400">
                            <div>Version: {diagnosticsData()?.version || 'Unknown'}</div>
                            <div>Uptime: {Math.floor((diagnosticsData()?.uptime || 0) / 60)} minutes</div>
                            <div>Runtime: {diagnosticsData()?.system?.goVersion || 'Unknown'}</div>
                            <div>Memory: {diagnosticsData()?.system?.memoryMB || 0} MB</div>
                          </div>
                        </div>
                        
                        {/* Nodes Status */}
                        <Show when={diagnosticsData()?.nodes?.length > 0}>
                          <div class="bg-white dark:bg-gray-800 rounded-lg p-3">
                            <h5 class="text-sm font-semibold mb-2 text-gray-700 dark:text-gray-300">PVE Nodes</h5>
                            <For each={diagnosticsData()?.nodes || []}>
                              {(node: any) => (
                                <div class="text-xs border-t dark:border-gray-700 pt-2 mt-2 first:border-0 first:pt-0 first:mt-0">
                                  <div class="flex justify-between items-center mb-1">
                                    <span class="font-medium text-gray-700 dark:text-gray-300">{node.name}</span>
                                    <span class={`px-2 py-0.5 rounded text-white text-xs ${
                                      node.connected ? 'bg-green-500' : 'bg-red-500'
                                    }`}>
                                      {node.connected ? 'Connected' : 'Failed'}
                                    </span>
                                  </div>
                                  <div class="text-gray-600 dark:text-gray-400">
                                    <div>Host: {node.host}</div>
                                    <div>Auth: {node.authMethod?.replace('_', ' ')}</div>
                                    <Show when={node.error}>
                                      <div class="text-red-500 mt-1 break-words">{node.error}</div>
                                    </Show>
                                  </div>
                                </div>
                              )}
                            </For>
                          </div>
                        </Show>
                        
                        {/* PBS Status */}
                        <Show when={diagnosticsData()?.pbs?.length > 0}>
                          <div class="bg-white dark:bg-gray-800 rounded-lg p-3">
                            <h5 class="text-sm font-semibold mb-2 text-gray-700 dark:text-gray-300">PBS Instances</h5>
                            <For each={diagnosticsData()?.pbs || []}>
                              {(pbs: any) => (
                                <div class="text-xs border-t dark:border-gray-700 pt-2 mt-2 first:border-0 first:pt-0 first:mt-0">
                                  <div class="flex justify-between items-center mb-1">
                                    <span class="font-medium text-gray-700 dark:text-gray-300">{pbs.name}</span>
                                    <span class={`px-2 py-0.5 rounded text-white text-xs ${
                                      pbs.connected ? 'bg-green-500' : 'bg-red-500'
                                    }`}>
                                      {pbs.connected ? 'Connected' : 'Failed'}
                                    </span>
                                  </div>
                                  <Show when={pbs.error}>
                                    <div class="text-red-500 break-words">{pbs.error}</div>
                                  </Show>
                                </div>
                              )}
                            </For>
                          </div>
                        </Show>
                      </div>
                    </Show>
                  </div>
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
                      <div class="flex justify-between">
                        <span class="text-gray-600 dark:text-gray-400">Backend Port:</span>
                        <span class="font-medium">{window.location.hostname === 'localhost' ? '3000' : '3000'}</span>
                      </div>
                      <div class="flex justify-between">
                        <span class="text-gray-600 dark:text-gray-400">Frontend Port:</span>
                        <span class="font-medium">{window.location.port || (window.location.protocol === 'https:' ? '443' : '80')}</span>
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
                            url: `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws`
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
        editingNode={editingNode() ?? undefined}
        onSave={async (nodeData) => {
          try {
            if (editingNode()) {
              // Update existing node
              await NodesAPI.updateNode(editingNode()!.id, nodeData as NodeConfig);
              
              // Update local state
              setNodes(nodes().map(n => 
                n.id === editingNode()!.id 
                  ? { 
                      ...n, 
                      ...nodeData, 
                      // Update hasPassword/hasToken based on whether credentials were provided
                      hasPassword: nodeData.password ? true : n.hasPassword,
                      hasToken: nodeData.tokenValue ? true : n.hasToken
                    }
                  : n
              ));
              showSuccess('Node updated successfully');
            } else {
              // Add new node
              await NodesAPI.addNode(nodeData as NodeConfig);
              
              // Reload nodes to get the new ID
              const nodesList = await NodesAPI.getNodes();
              const nodesWithStatus = nodesList.map(node => ({
                ...node,
                // Use the hasPassword/hasToken from the API if available, otherwise check local fields
                hasPassword: node.hasPassword ?? !!node.password,
                hasToken: node.hasToken ?? !!node.tokenValue,
                status: node.status || 'disconnected' as const
              }));
              setNodes(nodesWithStatus);
              showSuccess('Node added successfully');
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