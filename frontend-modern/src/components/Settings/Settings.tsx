import { Component, createSignal, onMount, For, Show, createEffect, onCleanup } from 'solid-js';
import { useWebSocket } from '@/App';
import { showSuccess, showError } from '@/utils/toast';
import { NodeModal } from './NodeModal';
import { GenerateAPIToken } from './GenerateAPIToken';
import { ChangePasswordModal } from './ChangePasswordModal';
import { GuestURLs } from './GuestURLs';
import { SettingsAPI } from '@/api/settings';
import { NodesAPI } from '@/api/nodes';
import { UpdatesAPI } from '@/api/updates';
import type { NodeConfig } from '@/types/nodes';
import type { UpdateInfo, VersionInfo } from '@/api/updates';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { updateStore } from '@/stores/updates';

// Type definitions
interface DiscoveredServer {
  ip: string;
  port: number;
  type: 'pve' | 'pbs';
  version: string;
  hostname?: string;
  release?: string;
}

interface ClusterEndpoint {
  Host?: string;
  IP?: string;
}

interface DiagnosticsNode {
  id: string;
  name: string;
  host: string;
  type: string;
  authMethod: string;
  connected: boolean;
  error?: string;
  details?: Record<string, unknown>;
  lastPoll?: string;
  clusterInfo?: Record<string, unknown>;
}

interface DiagnosticsPBS {
  id: string;
  name: string;
  host: string;
  connected: boolean;
  error?: string;
  details?: Record<string, unknown>;
}

interface SystemDiagnostic {
  goroutines: number;
  memory: {
    alloc: number;
    totalAlloc: number;
    sys: number;
    numGC: number;
  };
  cpu: {
    count: number;
    percent: number;
  };
}

interface DiagnosticsData {
  version: string;
  runtime: string;
  uptime: number;
  nodes: DiagnosticsNode[];
  pbs: DiagnosticsPBS[];
  system: SystemDiagnostic;
  errors: string[];
}

type SettingsTab = 'pve' | 'pbs' | 'system' | 'urls' | 'security' | 'diagnostics';

// Node with UI-specific fields
type NodeConfigWithStatus = NodeConfig & {
  hasPassword?: boolean;
  hasToken?: boolean;
  status: 'connected' | 'disconnected' | 'error';
};

const Settings: Component = () => {
  const { state, connected } = useWebSocket();
  const [activeTab, setActiveTab] = createSignal<SettingsTab>('pve');
  const [hasUnsavedChanges, setHasUnsavedChanges] = createSignal(false);
  const [nodes, setNodes] = createSignal<NodeConfigWithStatus[]>([]);
  const [discoveredNodes, setDiscoveredNodes] = createSignal<DiscoveredServer[]>([]);
  const [showNodeModal, setShowNodeModal] = createSignal(false);
  const [editingNode, setEditingNode] = createSignal<NodeConfigWithStatus | null>(null);
  const [currentNodeType, setCurrentNodeType] = createSignal<'pve' | 'pbs'>('pve');
  const [modalResetKey, setModalResetKey] = createSignal(0);
  const [showPasswordModal, setShowPasswordModal] = createSignal(false);
  const [initialLoadComplete, setInitialLoadComplete] = createSignal(false);
  
  // System settings
  // PBS polling interval removed - fixed at 10 seconds
  const [allowedOrigins, setAllowedOrigins] = createSignal('*');
  const [discoveryEnabled, setDiscoveryEnabled] = createSignal(true);
  const [discoverySubnet, setDiscoverySubnet] = createSignal('auto');
  const [envOverrides, setEnvOverrides] = createSignal<Record<string, boolean>>({});
  // Connection timeout removed - backend-only setting
  
  // Iframe embedding settings
  const [allowEmbedding, setAllowEmbedding] = createSignal(false);
  const [allowedEmbedOrigins, setAllowedEmbedOrigins] = createSignal('');
  
  // Update settings
  const [versionInfo, setVersionInfo] = createSignal<VersionInfo | null>(null);
  const [updateInfo, setUpdateInfo] = createSignal<UpdateInfo | null>(null);
  const [checkingForUpdates, setCheckingForUpdates] = createSignal(false);
  const [updateChannel, setUpdateChannel] = createSignal<'stable' | 'rc'>('stable');
  const [autoUpdateEnabled, setAutoUpdateEnabled] = createSignal(false);
  const [autoUpdateCheckInterval, setAutoUpdateCheckInterval] = createSignal(24);
  const [autoUpdateTime, setAutoUpdateTime] = createSignal('03:00');
  
  // Diagnostics
  const [diagnosticsData, setDiagnosticsData] = createSignal<DiagnosticsData | null>(null);
  const [runningDiagnostics, setRunningDiagnostics] = createSignal(false);
  
  // Security
  const [securityStatus, setSecurityStatus] = createSignal<{
    apiTokenConfigured: boolean;
    apiTokenHint?: string;
    requiresAuth: boolean;
    exportProtected: boolean;
    unprotectedExportAllowed: boolean;
    hasAuthentication: boolean;
    configuredButPendingRestart?: boolean;
    hasAuditLogging: boolean;
    credentialsEncrypted: boolean;
    hasHTTPS: boolean;
  } | null>(null);
  const [securityStatusLoading, setSecurityStatusLoading] = createSignal(true);
  const [exportPassphrase, setExportPassphrase] = createSignal('');
  const [useCustomPassphrase, setUseCustomPassphrase] = createSignal(false);
  const [importPassphrase, setImportPassphrase] = createSignal('');
  const [importFile, setImportFile] = createSignal<File | null>(null);
  const [showExportDialog, setShowExportDialog] = createSignal(false);
  const [showImportDialog, setShowImportDialog] = createSignal(false);
  const [showApiTokenModal, setShowApiTokenModal] = createSignal(false);
  const [apiTokenInput, setApiTokenInput] = createSignal('');
  const [apiTokenModalSource, setApiTokenModalSource] = createSignal<'export' | 'import' | null>(null);

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
      id: 'urls',
      label: 'Guest URLs',
      icon: 'M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1'
    },
    { 
      id: 'security', 
      label: 'Security',
      icon: 'M12 2L3.5 7v6c0 4.67 3.5 9.03 8.5 10 5-.97 8.5-5.33 8.5-10V7L12 2z'
    },
    { 
      id: 'diagnostics', 
      label: 'Diagnostics',
      icon: 'M3 3v18h18m-10-8l3-3 3 3 4-4'
    }
  ];

  // Function to load nodes
  const loadNodes = async () => {
    try {
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
    } catch (error) {
      console.error('Failed to load nodes:', error);
      // If we get a 429 or network error, retry after a delay
      if (error instanceof Error && (error.message.includes('429') || error.message.includes('fetch'))) {
        console.log('Retrying node load after delay...');
        setTimeout(() => loadNodes(), 3000);
      }
    }
  };
  
  // Function to load discovered nodes
  const loadSecurityStatus = async () => {
    setSecurityStatusLoading(true);
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/security/status');
      if (response.ok) {
        const status = await response.json();
        setSecurityStatus(status);
      }
    } catch (err) {
      console.error('Failed to fetch security status:', err);
    } finally {
      setSecurityStatusLoading(false);
    }
  };

  const loadDiscoveredNodes = async () => {
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/discover');
      if (response.ok) {
        const data = await response.json();
        if (data.servers && Array.isArray(data.servers)) {
          // Get all configured hosts and cluster member IPs
          const configuredHosts = new Set<string>();
          const clusterMemberIPs = new Set<string>();
          
          nodes().forEach(n => {
            // Add the main host
            const host = n.host.replace(/^https?:\/\//, '').replace(/:\d+$/, '');
            configuredHosts.add(host.toLowerCase());
            
            // If it's a cluster, add all member IPs
            if (n.type === 'pve' && 'isCluster' in n && n.isCluster && 'clusterEndpoints' in n && n.clusterEndpoints) {
              n.clusterEndpoints.forEach((endpoint: ClusterEndpoint) => {
                if (endpoint.IP) {
                  clusterMemberIPs.add(endpoint.IP.toLowerCase());
                }
                if (endpoint.Host) {
                  clusterMemberIPs.add(endpoint.Host.toLowerCase());
                }
              });
            }
          });
          
          // Filter out nodes that are already configured or part of a cluster
          const filtered = data.servers.filter((server: DiscoveredServer) => {
            const serverIP = server.ip?.toLowerCase();
            const serverHostname = server.hostname?.toLowerCase();
            
            // Check if this server is already configured directly
            if ((serverIP && configuredHosts.has(serverIP)) || (serverHostname && configuredHosts.has(serverHostname))) {
              return false;
            }
            
            // Check if this server is part of a configured cluster
            if ((serverIP && clusterMemberIPs.has(serverIP)) || (serverHostname && clusterMemberIPs.has(serverHostname))) {
              return false;
            }
            
            return true;
          });
          
          setDiscoveredNodes(filtered);
        }
      }
    } catch (error) {
      console.error('Failed to load discovered nodes:', error);
    }
  };

  // Load nodes and system settings on mount
  onMount(async () => {
    // Subscribe to events
    const unsubscribeAutoRegister = eventBus.on('node_auto_registered', () => {
      // Close any open modals
      setShowNodeModal(false);
      setEditingNode(null);
      // Reload nodes
      loadNodes();
      loadDiscoveredNodes();
    });
    
    const unsubscribeRefresh = eventBus.on('refresh_nodes', () => {
      loadNodes();
    });
    
    const unsubscribeDiscovery = eventBus.on('discovery_updated', (data) => {
      // If this is an immediate update (from node deletion), merge with existing
      if (data && data.immediate && data.servers) {
        setDiscoveredNodes(prev => {
          // Create a map of existing servers by IP:port
          const existingMap = new Map(prev.map(s => [`${s.ip}:${s.port}`, s]));
          
          // Add/update the new servers
          data.servers.forEach((server) => {
            const discoveredServer: DiscoveredServer = {
              ...server,
              type: server.type as 'pbs' | 'pve'
            };
            existingMap.set(`${server.ip}:${server.port}`, discoveredServer);
          });
          
          // Convert back to array
          return Array.from(existingMap.values());
        });
      } else {
        // Full discovery update - reload from API
        loadDiscoveredNodes();
      }
    });
    
    // Poll for node updates when modal is open
    let pollInterval: ReturnType<typeof setInterval> | undefined;
    createEffect(() => {
      // Clear any existing interval first
      if (pollInterval) {
        clearInterval(pollInterval);
        pollInterval = undefined;
      }
      
      if (showNodeModal()) {
        // Start polling every 3 seconds when modal is open
        pollInterval = setInterval(() => {
          loadNodes();
          loadDiscoveredNodes();
        }, 3000);
      }
    });
    
    // Poll for discovered nodes every 30 seconds
    const discoveryInterval = setInterval(() => {
      loadDiscoveredNodes();
    }, 30000);
    
    // Clean up on unmount
    onCleanup(() => {
      unsubscribeAutoRegister();
      unsubscribeRefresh();
      unsubscribeDiscovery();
      if (pollInterval) {
        clearInterval(pollInterval);
      }
      clearInterval(discoveryInterval);
    });
    
    try {
      // Load data with small delays to prevent rate limit bursts
      // Load security status first as it's lightweight
      await loadSecurityStatus();
      
      // Small delay to prevent burst
      await new Promise(resolve => setTimeout(resolve, 50));
      
      // Load nodes
      await loadNodes();
      
      // Another small delay
      await new Promise(resolve => setTimeout(resolve, 50));
      
      // Load discovered nodes
      await loadDiscoveredNodes();
      
      // Load system settings
      try {
        const systemResponse = await fetch('/api/config/system');
        if (systemResponse.ok) {
          const systemSettings = await systemResponse.json();
          // PBS polling interval is now fixed at 10 seconds
          setAllowedOrigins(systemSettings.allowedOrigins || '*');
          // Connection timeout is backend-only
          // Load discovery settings
          // Backend defaults to true, so we should respect that
          setDiscoveryEnabled(systemSettings.discoveryEnabled ?? true);  // Default to true if undefined
          setDiscoverySubnet(systemSettings.discoverySubnet || 'auto');
          // Load embedding settings
          setAllowEmbedding(systemSettings.allowEmbedding ?? false);
          setAllowedEmbedOrigins(systemSettings.allowedEmbedOrigins || '');
          // Load auto-update settings
          setAutoUpdateEnabled(systemSettings.autoUpdateEnabled || false);
          setAutoUpdateCheckInterval(systemSettings.autoUpdateCheckInterval || 24);
          setAutoUpdateTime(systemSettings.autoUpdateTime || '03:00');
          if (systemSettings.updateChannel) {
            setUpdateChannel(systemSettings.updateChannel as 'stable' | 'rc');
          }
          // Track environment variable overrides
          if (systemSettings.envOverrides) {
            setEnvOverrides(systemSettings.envOverrides);
          }
        } else {
          // Fallback to old endpoint
          await SettingsAPI.getSettings();
        }
      } catch (error) {
        console.error('Failed to load settings:', error);
      }
      
      // Load version information
      try {
        const version = await UpdatesAPI.getVersion();
        setVersionInfo(version);
        // Also set it in the store so it's available globally
        updateStore.checkForUpdates(); // This will load version info too
        if (version.channel) {
          setUpdateChannel(version.channel as 'stable' | 'rc');
        }
      } catch (error) {
        console.error('Failed to load version:', error);
      }
    } catch (error) {
      console.error('Failed to load configuration:', error);
    } finally {
      // Mark initial load as complete even if there were errors
      setInitialLoadComplete(true);
    }
  });

  const saveSettings = async () => {
    try {
      if (activeTab() === 'system') {
        // Save system settings using typed API
        await SettingsAPI.updateSystemSettings({
          // PBS polling interval is now fixed at 10 seconds
          allowedOrigins: allowedOrigins(),
          // Connection timeout is backend-only
          // Discovery settings are saved immediately on toggle
          updateChannel: updateChannel(),
          autoUpdateEnabled: autoUpdateEnabled(),
          autoUpdateCheckInterval: autoUpdateCheckInterval(),
          autoUpdateTime: autoUpdateTime(),
          allowEmbedding: allowEmbedding(),
          allowedEmbedOrigins: allowedEmbedOrigins()
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
      // Force check with current channel selection
      await updateStore.checkForUpdates(true);
      const info = updateStore.updateInfo();
      setUpdateInfo(info);
      
      // If update was dismissed, clear it so user can see it again
      if (info?.available && updateStore.isDismissed()) {
        updateStore.clearDismissed();
      }
      
      if (!info?.available) {
        showSuccess('You are running the latest version');
      }
    } catch (error) {
      showError('Failed to check for updates');
      console.error('Update check error:', error);
    } finally {
      setCheckingForUpdates(false);
    }
  };
  
  const handleExport = async () => {
    if (!exportPassphrase()) {
      const hasAuth = securityStatus()?.hasAuthentication;
      showError(hasAuth 
        ? (useCustomPassphrase() ? 'Please enter a passphrase' : 'Please enter your password')
        : 'Please enter a passphrase');
      return;
    }
    
    // Just require a passphrase to be entered
    const hasAuth = securityStatus()?.hasAuthentication;
    if ((!hasAuth || useCustomPassphrase()) && !exportPassphrase()) {
      showError('Please enter a passphrase');
      return;
    }
    
    // Only check for API token if user is not authenticated via password
    // If user is logged in with password, session auth is sufficient
    const hasPasswordAuth = securityStatus()?.hasAuthentication;
    if (!hasPasswordAuth && securityStatus()?.apiTokenConfigured && !localStorage.getItem('apiToken')) {
      setApiTokenModalSource('export');
      setShowApiTokenModal(true);
      return;
    }
    
    try {
      // Get CSRF token from cookie
      const csrfToken = document.cookie
        .split('; ')
        .find(row => row.startsWith('pulse_csrf='))
        ?.split('=')[1];
      
      const headers: HeadersInit = {
        'Content-Type': 'application/json',
      };
      
      // Add CSRF token if available
      if (csrfToken) {
        headers['X-CSRF-Token'] = csrfToken;
      }
      
      // Add API token if configured
      const apiToken = localStorage.getItem('apiToken');
      if (apiToken) {
        headers['X-API-Token'] = apiToken;
      }
      
      const response = await fetch('/api/config/export', {
        method: 'POST',
        headers,
        credentials: 'include', // Include cookies for session auth
        body: JSON.stringify({ passphrase: exportPassphrase() }),
      });
      
      if (!response.ok) {
        const errorText = await response.text();
        // Handle authentication errors
        if (response.status === 401 || response.status === 403) {
          // Check if we're using API token auth (not password auth)
          const hasPasswordAuth = securityStatus()?.hasAuthentication;
          if (!hasPasswordAuth) {
            // Clear invalid token if we had one
            const hadToken = localStorage.getItem('apiToken');
            if (hadToken) {
              localStorage.removeItem('apiToken');
              showError('Invalid or expired API token. Please re-enter.');
              setApiTokenModalSource('export');
              setShowApiTokenModal(true);
              return;
            }
            if (errorText.includes('API_TOKEN')) {
              setApiTokenModalSource('export');
              setShowApiTokenModal(true);
              return;
            }
          }
          throw new Error('Export requires authentication');
        }
        throw new Error(errorText || 'Export failed');
      }
      
      const data = await response.json();
      
      // Create and download file
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `pulse-config-${new Date().toISOString().split('T')[0]}.json`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      
      showSuccess('Configuration exported successfully');
      setShowExportDialog(false);
      setExportPassphrase('');
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Failed to export configuration';
      showError(errorMessage);
      console.error('Export error:', error);
    }
  };
  
  const handleImport = async () => {
    if (!importPassphrase()) {
      showError('Please enter the password');
      return;
    }
    
    if (!importFile()) {
      showError('Please select a file to import');
      return;
    }
    
    // Only check for API token if user is not authenticated via password
    // If user is logged in with password, session auth is sufficient
    const hasPasswordAuth = securityStatus()?.hasAuthentication;
    if (!hasPasswordAuth && securityStatus()?.apiTokenConfigured && !localStorage.getItem('apiToken')) {
      setApiTokenModalSource('import');
      setShowApiTokenModal(true);
      return;
    }
    
    try {
      const fileContent = await importFile()!.text();
      let exportData;
      try {
        exportData = JSON.parse(fileContent);
      } catch (parseError) {
        showError('Invalid JSON file format');
        console.error('JSON parse error:', parseError);
        return;
      }
      
      // Get CSRF token from cookie
      const csrfToken = document.cookie
        .split('; ')
        .find(row => row.startsWith('pulse_csrf='))
        ?.split('=')[1];
      
      const headers: HeadersInit = {
        'Content-Type': 'application/json',
      };
      
      // Add CSRF token if available
      if (csrfToken) {
        headers['X-CSRF-Token'] = csrfToken;
      }
      
      // Add API token if configured
      const apiToken = localStorage.getItem('apiToken');
      if (apiToken) {
        headers['X-API-Token'] = apiToken;
      }
      
      const response = await fetch('/api/config/import', {
        method: 'POST',
        headers,
        credentials: 'include', // Include cookies for session auth
        body: JSON.stringify({
          passphrase: importPassphrase(),
          data: exportData.data,
        }),
      });
      
      if (!response.ok) {
        const errorText = await response.text();
        // Handle authentication errors
        if (response.status === 401 || response.status === 403) {
          // Check if we're using API token auth (not password auth)
          const hasPasswordAuth = securityStatus()?.hasAuthentication;
          if (!hasPasswordAuth) {
            // Clear invalid token if we had one
            const hadToken = localStorage.getItem('apiToken');
            if (hadToken) {
              localStorage.removeItem('apiToken');
              showError('Invalid or expired API token. Please re-enter.');
              setApiTokenModalSource('import');
              setShowApiTokenModal(true);
              return;
            }
            if (errorText.includes('API_TOKEN')) {
              setApiTokenModalSource('import');
              setShowApiTokenModal(true);
              return;
            }
          }
          throw new Error('Import requires authentication');
        }
        throw new Error(errorText || 'Import failed');
      }
      
      showSuccess('Configuration imported successfully. Reloading...');
      setShowImportDialog(false);
      setImportPassphrase('');
      setImportFile(null);
      
      // Reload page to apply new configuration
      setTimeout(() => window.location.reload(), 2000);
    } catch (error) {
      showError('Failed to import configuration');
      console.error('Import error:', error);
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
              <button type="button" 
                class="flex-1 sm:flex-initial px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                onClick={saveSettings}
              >
                Save Changes
              </button>
              <button type="button" 
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
                <button type="button"
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
              <Show when={!initialLoadComplete()}>
                <div class="flex items-center justify-center py-8">
                  <span class="text-gray-500">Loading configuration...</span>
                </div>
              </Show>
              <Show when={initialLoadComplete()}>
              <div class="flex items-center justify-between mb-4">
                <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Proxmox VE Nodes</h3>
                <div class="flex gap-2 items-center">
                  {/* Discovery toggle */}
                  <label class="flex items-center gap-2 cursor-pointer" title="Enable automatic discovery of Proxmox servers on your network">
                    <span class="text-sm text-gray-600 dark:text-gray-400">Discovery</span>
                    <div class="relative inline-flex items-center">
                      <input
                        type="checkbox"
                        checked={discoveryEnabled()}
                        onChange={async (e) => {
                          if (!envOverrides().discoveryEnabled) {
                            const newValue = e.currentTarget.checked;
                            setDiscoveryEnabled(newValue);
                            
                            // Save discovery setting immediately
                            try {
                              await SettingsAPI.updateSystemSettings({
                                discoveryEnabled: newValue,
                                discoverySubnet: discoverySubnet()
                              });
                              
                              if (newValue) {
                                // Trigger discovery when enabled
                                loadDiscoveredNodes();
                                notificationStore.success('Discovery enabled', 2000);
                              } else {
                                notificationStore.info('Discovery disabled', 2000);
                              }
                            } catch (error) {
                              console.error('Failed to update discovery setting:', error);
                              notificationStore.error('Failed to update discovery setting');
                              // Revert on error
                              setDiscoveryEnabled(!newValue);
                            }
                          }
                        }}
                        disabled={envOverrides().discoveryEnabled}
                        class="sr-only peer"
                      />
                      <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                    </div>
                  </label>
                  
                  <Show when={discoveryEnabled()}>
                    <button type="button" 
                      onClick={() => {
                        loadDiscoveredNodes();
                        notificationStore.info('Refreshing discovery...', 2000);
                      }}
                      class="px-4 py-2 text-sm bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors flex items-center gap-2"
                      title="Refresh discovered servers"
                    >
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="23 4 23 10 17 10"></polyline>
                        <polyline points="1 20 1 14 7 14"></polyline>
                        <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"></path>
                      </svg>
                      Refresh
                    </button>
                  </Show>
                  
                  <button type="button" 
                    onClick={() => {
                      setEditingNode(null);
                      setCurrentNodeType('pve');
                      setModalResetKey(prev => prev + 1);
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
              </div>
              
              <div class="grid gap-4">
                <For each={nodes().filter(n => n.type === 'pve')}>
                  {(node) => (
                    <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4 border border-gray-200 dark:border-gray-600">
                      <div class="flex items-start justify-between">
                        <div class="flex items-start gap-3">
                          <div class="relative">
                            <div class={`w-3 h-3 rounded-full mt-1.5 ${
                            (() => {
                              // Find the corresponding node in the WebSocket state
                              const stateNode = state.nodes.find(n => n.instance === node.name);
                              // Check if the node has an unhealthy connection or is offline
                              if (stateNode?.connectionHealth === 'unhealthy' || stateNode?.status === 'offline') {
                                return 'bg-red-500';
                              }
                              // Check if we have a healthy connection
                              if (stateNode && stateNode.status === 'online') {
                                return 'bg-green-500';
                              }
                              // Default to red if no state data (node is offline/unreachable)
                              return 'bg-red-500';
                            })()
                          }`}></div>
                          </div>
                          <div class="flex-1">
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
                              <div class="mt-3 p-3 bg-gray-100 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
                                <div class="flex items-center gap-2 mb-2">
                                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" class="text-gray-600 dark:text-gray-400">
                                    <circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="2" fill="none"/>
                                    <circle cx="4" cy="12" r="2" stroke="currentColor" stroke-width="2" fill="none"/>
                                    <circle cx="20" cy="12" r="2" stroke="currentColor" stroke-width="2" fill="none"/>
                                    <line x1="7" y1="12" x2="9" y2="12" stroke="currentColor" stroke-width="2"/>
                                    <line x1="15" y1="12" x2="17" y2="12" stroke="currentColor" stroke-width="2"/>
                                  </svg>
                                  <span class="font-semibold text-gray-700 dark:text-gray-300">
                                    {'clusterName' in node ? node.clusterName : 'Unknown'} Cluster
                                  </span>
                                  <span class="text-xs bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-400 px-2 py-0.5 rounded-full ml-auto">
                                    {'clusterEndpoints' in node && node.clusterEndpoints ? node.clusterEndpoints.length : 0} nodes
                                  </span>
                                </div>
                                <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                                  <For each={'clusterEndpoints' in node ? node.clusterEndpoints : []}>
                                    {(endpoint) => (
                                      <div class="flex items-center gap-2 text-xs bg-white dark:bg-gray-900 px-2 py-1.5 rounded border border-gray-200 dark:border-gray-700">
                                        <div class={`w-2 h-2 rounded-full flex-shrink-0 ${endpoint.Online ? 'bg-green-500' : 'bg-gray-400'}`}></div>
                                        <span class="font-medium text-gray-700 dark:text-gray-300">{endpoint.NodeName}</span>
                                        <span class="text-gray-500 dark:text-gray-500 ml-auto">{endpoint.IP}</span>
                                      </div>
                                    )}
                                  </For>
                                </div>
                                <p class="mt-2 text-xs text-gray-600 dark:text-gray-400 flex items-center gap-1">
                                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                    <path d="M5 12h14M12 5l7 7-7 7"/>
                                  </svg>
                                  Automatic failover enabled between cluster nodes
                                </p>
                              </div>
                            </Show>
                          </div>
                        </div>
                        <div class="flex items-center gap-2">
                          <button type="button"
                            onClick={() => testNodeConnection(node.id)}
                            class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                            title="Test connection"
                          >
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                              <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
                            </svg>
                          </button>
                          <button type="button"
                            onClick={() => {
                              setEditingNode(node);
                              setCurrentNodeType(node.type as 'pve' | 'pbs');
                              setShowNodeModal(true);
                            }}
                            class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                          >
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                              <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"></path>
                              <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                            </svg>
                          </button>
                          <button type="button"
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
                
                {nodes().filter(n => n.type === 'pve').length === 0 && discoveredNodes().filter(n => n.type === 'pve').length === 0 && (
                  <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                    <p>No PVE nodes configured</p>
                    <p class="text-sm mt-1">Add a node to start monitoring</p>
                  </div>
                )}
                
                {/* Discovered PVE nodes - only show when discovery is enabled */}
                <Show when={discoveryEnabled()}>
                  <For each={discoveredNodes().filter(n => n.type === 'pve')}>
                    {(server) => (
                    <div 
                      class="bg-gray-50/50 dark:bg-gray-700/30 rounded-lg p-4 border border-gray-200/50 dark:border-gray-600/50 opacity-75 hover:opacity-100 transition-opacity cursor-pointer"
                      onClick={() => {
                        // Pre-fill the modal with discovered server info
                        setEditingNode({
                          id: '',
                          type: 'pve',
                          name: server.hostname || `pve-${server.ip}`,
                          host: `https://${server.ip}:${server.port}`,
                          tokenName: '',
                          tokenValue: '',
                          verifySSL: false,
                          monitorVMs: true,
                          monitorContainers: true,
                          monitorStorage: true,
                          monitorBackups: true,
                          status: 'disconnected'
                        } as NodeConfigWithStatus);
                        setCurrentNodeType('pve');
                        setShowNodeModal(true);
                      }}
                    >
                      <div class="flex items-start justify-between">
                        <div class="flex items-start gap-3">
                          <div class="relative">
                            <div class="w-3 h-3 rounded-full mt-1.5 bg-gray-400 animate-pulse"></div>
                          </div>
                          <div class="flex-1">
                            <h4 class="font-medium text-gray-700 dark:text-gray-300">
                              {server.hostname || `Proxmox VE at ${server.ip}`}
                            </h4>
                            <p class="text-sm text-gray-500 dark:text-gray-500 mt-1">
                              {server.ip}:{server.port}
                            </p>
                            <div class="flex items-center gap-2 mt-2">
                              <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded">
                                Discovered
                              </span>
                              <span class="text-xs text-gray-500 dark:text-gray-400">
                                Click to configure
                              </span>
                            </div>
                          </div>
                        </div>
                        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" class="text-gray-400 mt-1">
                          <path d="M12 5v14m-7-7h14" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                        </svg>
                      </div>
                    </div>
                  )}
                </For>
                </Show>
              </div>
              </Show>
            </div>
          </Show>
          
          {/* PBS Nodes Tab */}
          <Show when={activeTab() === 'pbs'}>
            <div class="space-y-4">
              <Show when={!initialLoadComplete()}>
                <div class="flex items-center justify-center py-8">
                  <span class="text-gray-500">Loading configuration...</span>
                </div>
              </Show>
              <Show when={initialLoadComplete()}>
              <div class="flex items-center justify-between mb-4">
                <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Proxmox Backup Server Nodes</h3>
                <div class="flex gap-2 items-center">
                  {/* Discovery toggle */}
                  <label class="flex items-center gap-2 cursor-pointer" title="Enable automatic discovery of PBS servers on your network">
                    <span class="text-sm text-gray-600 dark:text-gray-400">Discovery</span>
                    <div class="relative inline-flex items-center">
                      <input
                        type="checkbox"
                        checked={discoveryEnabled()}
                        onChange={async (e) => {
                          if (!envOverrides().discoveryEnabled) {
                            const newValue = e.currentTarget.checked;
                            setDiscoveryEnabled(newValue);
                            
                            // Save discovery setting immediately
                            try {
                              await SettingsAPI.updateSystemSettings({
                                discoveryEnabled: newValue,
                                discoverySubnet: discoverySubnet()
                              });
                              
                              if (newValue) {
                                // Trigger discovery when enabled
                                loadDiscoveredNodes();
                                notificationStore.success('Discovery enabled', 2000);
                              } else {
                                notificationStore.info('Discovery disabled', 2000);
                              }
                            } catch (error) {
                              console.error('Failed to update discovery setting:', error);
                              notificationStore.error('Failed to update discovery setting');
                              // Revert on error
                              setDiscoveryEnabled(!newValue);
                            }
                          }
                        }}
                        disabled={envOverrides().discoveryEnabled}
                        class="sr-only peer"
                      />
                      <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                    </div>
                  </label>
                  
                  <Show when={discoveryEnabled()}>
                    <button type="button" 
                      onClick={() => {
                        loadDiscoveredNodes();
                        notificationStore.info('Refreshing discovery...', 2000);
                      }}
                      class="px-4 py-2 text-sm bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors flex items-center gap-2"
                      title="Refresh discovered servers"
                    >
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="23 4 23 10 17 10"></polyline>
                        <polyline points="1 20 1 14 7 14"></polyline>
                        <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"></path>
                      </svg>
                      Refresh
                    </button>
                  </Show>
                  
                  <button type="button" 
                    onClick={() => {
                      setEditingNode(null);
                      setCurrentNodeType('pbs');
                      setModalResetKey(prev => prev + 1);
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
                              // Check if the PBS has an unhealthy connection or is offline
                              if (statePBS?.connectionHealth === 'unhealthy' || statePBS?.status === 'offline') {
                                return 'bg-red-500';
                              }
                              // Check if we have a healthy connection
                              if (statePBS && statePBS.status === 'online') {
                                return 'bg-green-500';
                              }
                              // Default to red if no state data (server is offline/unreachable)
                              return 'bg-red-500';
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
                          <button type="button"
                            onClick={() => testNodeConnection(node.id)}
                            class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                            title="Test connection"
                          >
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                              <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
                            </svg>
                          </button>
                          <button type="button"
                            onClick={() => {
                              setEditingNode(node);
                              setCurrentNodeType(node.type as 'pve' | 'pbs');
                              setShowNodeModal(true);
                            }}
                            class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                          >
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                              <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"></path>
                              <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                            </svg>
                          </button>
                          <button type="button"
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
                
                {nodes().filter(n => n.type === 'pbs').length === 0 && discoveredNodes().filter(n => n.type === 'pbs').length === 0 && (
                  <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                    <p>No PBS nodes configured</p>
                    <p class="text-sm mt-1">Add a node to start monitoring</p>
                  </div>
                )}
                
                {/* Discovered PBS nodes - only show when discovery is enabled */}
                <Show when={discoveryEnabled()}>
                  <For each={discoveredNodes().filter(n => n.type === 'pbs')}>
                    {(server) => (
                    <div 
                      class="bg-gray-50/50 dark:bg-gray-700/30 rounded-lg p-4 border border-gray-200/50 dark:border-gray-600/50 opacity-75 hover:opacity-100 transition-opacity cursor-pointer"
                      onClick={() => {
                        // Pre-fill the modal with discovered server info
                        setEditingNode({
                          id: '',
                          type: 'pbs',
                          name: server.hostname || `pbs-${server.ip}`,
                          host: `https://${server.ip}:${server.port}`,
                          tokenName: '',
                          tokenValue: '',
                          verifySSL: false,
                          monitorDatastores: true,
                          monitorSyncJobs: true,
                          monitorVerifyJobs: true,
                          monitorPruneJobs: true,
                          status: 'disconnected'
                        } as NodeConfigWithStatus);
                        setCurrentNodeType('pbs');
                        setShowNodeModal(true);
                      }}
                    >
                      <div class="flex items-start justify-between">
                        <div class="flex items-start gap-3">
                          <div class="relative">
                            <div class="w-3 h-3 rounded-full mt-1.5 bg-gray-400 animate-pulse"></div>
                          </div>
                          <div class="flex-1">
                            <h4 class="font-medium text-gray-700 dark:text-gray-300">
                              {server.hostname || `Backup Server at ${server.ip}`}
                            </h4>
                            <p class="text-sm text-gray-500 dark:text-gray-500 mt-1">
                              {server.ip}:{server.port}
                            </p>
                            <div class="flex items-center gap-2 mt-2">
                              <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded">
                                Discovered
                              </span>
                              <span class="text-xs text-gray-500 dark:text-gray-400">
                                Click to configure
                              </span>
                            </div>
                          </div>
                        </div>
                        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" class="text-gray-400 mt-1">
                          <path d="M12 5v14m-7-7h14" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                        </svg>
                      </div>
                    </div>
                  )}
                </For>
                </Show>
              </div>
              </Show>
            </div>
          </Show>
          
          {/* System Settings Tab */}
          <Show when={activeTab() === 'system'}>
            <div class="space-y-6">
              <div>
                <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">System Configuration</h3>
                
                {/* Environment Variable Info */}
                <div class="mb-4 p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-700 rounded-lg">
                  <div class="flex items-start gap-2">
                    <svg class="w-5 h-5 text-blue-600 dark:text-blue-400 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <div class="text-sm text-blue-800 dark:text-blue-200">
                      <p class="font-medium mb-1">Configuration Priority:</p>
                      <p>• Some env vars override settings (API_TOKEN, PORTS, AUTH)</p>
                      <p>• Changes made here are saved to system.json immediately</p>
                      <p>• Settings persist unless overridden by env vars</p>
                    </div>
                  </div>
                </div>
                
                <div class="space-y-4">
                  
                  {/* Network Settings */}
                  <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4">
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4 flex items-center gap-2">
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <circle cx="12" cy="12" r="10"></circle>
                        <path d="M2 12h20M12 2a15.3 15.3 0 014 10 15.3 15.3 0 01-4 10 15.3 15.3 0 01-4-10 15.3 15.3 0 014-10z"></path>
                      </svg>
                      Network Settings
                    </h4>
                    
                    <div>
                      <label class="text-sm font-medium text-gray-900 dark:text-gray-100">CORS Allowed Origins</label>
                      <p class="text-xs text-gray-600 dark:text-gray-400 mb-2">For reverse proxy setups (* = allow all, empty = same-origin only)</p>
                      <div class="relative">
                        <input
                          type="text"
                          value={allowedOrigins()}
                          onChange={(e) => {
                            if (!envOverrides().allowedOrigins) {
                              setAllowedOrigins(e.currentTarget.value);
                              setHasUnsavedChanges(true);
                            }
                          }}
                          disabled={envOverrides().allowedOrigins}
                          placeholder="* or https://example.com"
                          class={`w-full px-3 py-1.5 text-sm border rounded-lg ${
                            envOverrides().allowedOrigins 
                              ? 'border-amber-300 dark:border-amber-600 bg-amber-50 dark:bg-amber-900/20 cursor-not-allowed opacity-75' 
                              : 'border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800'
                          }`}
                        />
                        {envOverrides().allowedOrigins && (
                          <div class="mt-2 p-2 bg-amber-100 dark:bg-amber-900/30 border border-amber-300 dark:border-amber-700 rounded text-xs text-amber-800 dark:text-amber-200">
                            <div class="flex items-center gap-1">
                              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                              </svg>
                              <span>Overridden by ALLOWED_ORIGINS environment variable</span>
                            </div>
                            <div class="mt-1 text-amber-700 dark:text-amber-300">
                              Remove the env var and restart to enable UI configuration
                            </div>
                          </div>
                        )}
                      </div>
                    </div>
                    
                    {/* Iframe Embedding Settings */}
                    <div class="mt-4">
                      <label class="text-sm font-medium text-gray-900 dark:text-gray-100">Iframe Embedding</label>
                      <p class="text-xs text-gray-600 dark:text-gray-400 mb-2">Allow Pulse to be embedded in iframes (e.g., Homepage dashboard)</p>
                      
                      <div class="space-y-3">
                        <div class="flex items-center gap-2">
                          <input
                            type="checkbox"
                            id="allowEmbedding"
                            checked={allowEmbedding()}
                            onChange={(e) => {
                              setAllowEmbedding(e.currentTarget.checked);
                              setHasUnsavedChanges(true);
                            }}
                            class="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
                          />
                          <label for="allowEmbedding" class="text-sm text-gray-700 dark:text-gray-300">
                            Allow iframe embedding
                          </label>
                        </div>
                        
                        <Show when={allowEmbedding()}>
                          <div>
                            <label class="text-xs font-medium text-gray-700 dark:text-gray-300">Allowed Embed Origins (optional)</label>
                            <p class="text-xs text-gray-600 dark:text-gray-400 mb-1">Comma-separated list of origins that can embed Pulse (leave empty for same-origin only)</p>
                            <input
                              type="text"
                              value={allowedEmbedOrigins()}
                              onChange={(e) => {
                                setAllowedEmbedOrigins(e.currentTarget.value);
                                setHasUnsavedChanges(true);
                              }}
                              placeholder="https://my.domain, https://dashboard.example.com"
                              class="w-full px-3 py-1.5 text-sm border rounded-lg border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800"
                            />
                            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                              Example: If Pulse is at <code>pulse.my.domain</code> and your dashboard is at <code>my.domain</code>, 
                              add <code>https://my.domain</code> here.
                            </p>
                          </div>
                        </Show>
                      </div>
                    </div>
                    
                    <div class="mt-3 p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg">
                      <p class="text-xs text-amber-800 dark:text-amber-200 mb-2">
                        <strong>Port Configuration:</strong> Use <code class="font-mono bg-amber-100 dark:bg-amber-800 px-1 rounded">systemctl edit pulse</code>
                      </p>
                      <p class="text-xs text-amber-700 dark:text-amber-300 font-mono">
                        [Service]<br/>
                        Environment="FRONTEND_PORT=8080"<br/>
                        <span class="text-xs text-amber-600 dark:text-amber-400">Then restart: sudo systemctl restart pulse</span>
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
                        <button type="button"
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
                      <Show when={versionInfo()?.isDocker && !updateInfo()?.available}>
                        <div class="p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
                          <p class="text-xs text-blue-800 dark:text-blue-200">
                            <strong>Docker Installation:</strong> Updates are managed through Docker. Pull the latest image to update.
                          </p>
                        </div>
                      </Show>
                      
                      {/* Update Available */}
                      <Show when={updateInfo()?.available}>
                        <div class="p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg">
                          <div class="mb-2">
                            <p class="text-sm font-medium text-green-800 dark:text-green-200">
                              Update Available: {updateInfo()?.latestVersion}
                            </p>
                            <p class="text-xs text-green-700 dark:text-green-300 mt-1">
                              Released: {updateInfo()?.releaseDate ? new Date(updateInfo()!.releaseDate).toLocaleDateString() : 'Unknown'}
                            </p>
                          </div>
                          
                          {/* Update Instructions based on deployment type */}
                          <div class="mt-3 p-2 bg-green-100 dark:bg-green-900/40 rounded">
                            <p class="text-xs font-medium text-green-800 dark:text-green-200 mb-1">How to update:</p>
                            <Show when={versionInfo()?.deploymentType === 'proxmoxve'}>
                              <p class="text-xs text-green-700 dark:text-green-300">
                                Type <code class="px-1 py-0.5 bg-green-200 dark:bg-green-800 rounded">update</code> in the LXC console
                              </p>
                            </Show>
                            <Show when={versionInfo()?.deploymentType === 'docker'}>
                              <div class="text-xs text-green-700 dark:text-green-300 space-y-1">
                                <p>Run these commands:</p>
                                <code class="block p-1 bg-green-200 dark:bg-green-800 rounded text-xs">
                                  docker pull rcourtman/pulse:latest<br/>
                                  docker restart pulse
                                </code>
                              </div>
                            </Show>
                            <Show when={versionInfo()?.deploymentType === 'systemd' || versionInfo()?.deploymentType === 'manual'}>
                              <div class="text-xs text-green-700 dark:text-green-300 space-y-1">
                                <p>Run the install script:</p>
                                <code class="block p-1 bg-green-200 dark:bg-green-800 rounded text-xs">
                                  curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
                                </code>
                              </div>
                            </Show>
                            <Show when={versionInfo()?.deploymentType === 'development'}>
                              <p class="text-xs text-green-700 dark:text-green-300">
                                Pull latest changes and rebuild
                              </p>
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
              
              {/* Backup & Restore - Moved from Security tab */}
              <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
                <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">Backup & Restore</h3>
                <p class="text-sm text-gray-600 dark:text-gray-400 mb-6">
                  Backup your node configurations and credentials or restore from a previous backup
                </p>
                
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                  {/* Export Section */}
                  <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                    <div class="flex items-start gap-3">
                      <div class="flex-shrink-0 w-10 h-10 bg-blue-100 dark:bg-blue-900/30 rounded-lg flex items-center justify-center">
                        <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10" />
                        </svg>
                      </div>
                      <div class="flex-1">
                        <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-1">Export Configuration</h4>
                        <p class="text-xs text-gray-600 dark:text-gray-400 mb-3">
                          Download an encrypted backup of all nodes and settings
                        </p>
                        <button type="button"
                          onClick={() => {
                            // Default to custom passphrase if no auth is configured
                            setUseCustomPassphrase(!securityStatus()?.hasAuthentication);
                            setShowExportDialog(true);
                          }}
                          class="px-3 py-1.5 bg-blue-600 text-white text-sm rounded-md hover:bg-blue-700 transition-colors inline-flex items-center gap-2"
                        >
                          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                          </svg>
                          Export Backup
                        </button>
                      </div>
                    </div>
                  </div>
                  
                  {/* Import Section */}
                  <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                    <div class="flex items-start gap-3">
                      <div class="flex-shrink-0 w-10 h-10 bg-gray-100 dark:bg-gray-700 rounded-lg flex items-center justify-center">
                        <svg class="w-5 h-5 text-gray-600 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                        </svg>
                      </div>
                      <div class="flex-1">
                        <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-1">Restore Configuration</h4>
                        <p class="text-xs text-gray-600 dark:text-gray-400 mb-3">
                          Upload a backup file to restore nodes and settings
                        </p>
                        <button type="button"
                          onClick={() => setShowImportDialog(true)}
                          class="px-3 py-1.5 bg-gray-600 text-white text-sm rounded-md hover:bg-gray-700 transition-colors inline-flex items-center gap-2"
                        >
                          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                          </svg>
                          Restore Backup
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
                
                <div class="mt-4 p-3 bg-amber-50 dark:bg-amber-900/20 rounded-lg border border-amber-200 dark:border-amber-800">
                  <div class="flex gap-2">
                    <svg class="w-4 h-4 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                    </svg>
                    <div class="text-xs text-amber-700 dark:text-amber-300">
                      <p class="font-medium mb-1">Important Notes</p>
                      <ul class="space-y-0.5 text-amber-600 dark:text-amber-400">
                        <li>• Backups contain encrypted credentials and sensitive data</li>
                        <li>• Use a strong passphrase to protect your backup</li>
                        <li>• Store backup files securely and never share the passphrase</li>
                      </ul>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </Show>
          
          {/* Security Tab */}
          <Show when={activeTab() === 'security'}>
            <div class="space-y-6">
              {/* Show message when auth is disabled */}
              <Show when={!securityStatus()?.hasAuthentication}>
                <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-6">
                  <div class="flex items-start space-x-3">
                    <div class="flex-shrink-0">
                      <svg class="h-6 w-6 text-amber-600 dark:text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                      </svg>
                    </div>
                    <div class="flex-1">
                      <h4 class="text-sm font-semibold text-amber-900 dark:text-amber-100">
                        Authentication is Disabled
                      </h4>
                      <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
                        Pulse is currently running without authentication. This means anyone who can access this interface has full control.
                      </p>
                      <div class="mt-4 bg-white dark:bg-gray-800 rounded-lg p-3 border border-amber-200 dark:border-amber-700">
                        <p class="text-xs font-semibold text-gray-900 dark:text-gray-100 mb-2">
                          Why is authentication disabled?
                        </p>
                        <ul class="text-xs text-gray-600 dark:text-gray-400 space-y-1">
                          <li>• DISABLE_AUTH environment variable is set to true</li>
                          <li>• Recovery mode is active (.auth_recovery file exists)</li>
                          <li>• Or authentication hasn't been configured yet</li>
                        </ul>
                      </div>
                      <div class="mt-3 bg-white dark:bg-gray-800 rounded-lg p-3 border border-amber-200 dark:border-amber-700">
                        <p class="text-xs font-semibold text-gray-900 dark:text-gray-100 mb-2">
                          To enable authentication:
                        </p>
                        <ol class="text-xs text-gray-600 dark:text-gray-400 space-y-1">
                          <li>1. Remove DISABLE_AUTH from environment variables</li>
                          <li>2. Delete /etc/pulse/.auth_recovery if it exists</li>
                          <li>3. Restart Pulse service</li>
                          <li>4. Complete the security setup wizard on first access</li>
                        </ol>
                      </div>
                    </div>
                  </div>
                </div>
              </Show>
              
              {/* Authentication */}
              <Show when={securityStatus()?.hasAuthentication}>
                <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
                  {/* Header */}
                  <div class="bg-gradient-to-r from-gray-50 to-gray-50 dark:from-gray-900/20 dark:to-gray-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                    <div class="flex items-center gap-3">
                      <div class="p-2 bg-gray-100 dark:bg-gray-900/50 rounded-lg">
                        <svg class="w-5 h-5 text-gray-600 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                        </svg>
                      </div>
                      <div>
                        <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-100">Authentication</h3>
                        <p class="text-xs text-gray-600 dark:text-gray-400">Manage your login credentials</p>
                      </div>
                    </div>
                  </div>
                  
                  {/* Content */}
                  <div class="p-6">
                    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <button type="button"
                        onClick={() => setShowPasswordModal(true)}
                        class="flex items-center gap-3 p-4 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-all group"
                      >
                        <div class="p-2 bg-blue-100 dark:bg-blue-900/30 rounded-lg group-hover:bg-blue-200 dark:group-hover:bg-blue-900/50 transition-colors">
                          <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
                          </svg>
                        </div>
                        <div class="text-left">
                          <div class="text-sm font-medium text-gray-900 dark:text-gray-100">Change Password</div>
                          <div class="text-xs text-gray-500 dark:text-gray-400">Update your login credentials</div>
                        </div>
                      </button>
                      
                    </div>
                  </div>
                </div>
              </Show>

              {/* Show pending restart message if configured but not loaded */}
              <Show when={securityStatus()?.configuredButPendingRestart}>
                <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-4">
                  <div class="flex items-start space-x-3">
                    <div class="flex-shrink-0">
                      <svg class="h-6 w-6 text-amber-600 dark:text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                      </svg>
                    </div>
                    <div class="flex-1">
                      <h4 class="text-sm font-semibold text-amber-900 dark:text-amber-100">
                        Security Configured - Restart Required
                      </h4>
                      <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
                        Security settings have been configured but the service needs to be restarted to activate them.
                      </p>
                      <p class="text-xs text-amber-600 dark:text-amber-400 mt-2">
                        After restarting, you'll need to log in with your saved credentials.
                      </p>
                      
                      <div class="mt-4 bg-white dark:bg-gray-800 rounded-lg p-3 border border-amber-200 dark:border-amber-700">
                        <p class="text-xs font-semibold text-gray-900 dark:text-gray-100 mb-2">
                          How to restart Pulse:
                        </p>
                        
                        <Show when={versionInfo()?.deploymentType === 'proxmoxve'}>
                          <div class="space-y-2">
                            <p class="text-xs text-gray-700 dark:text-gray-300">
                              Type <code class="px-1 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">update</code> in your ProxmoxVE console
                            </p>
                            <p class="text-xs text-gray-600 dark:text-gray-400 italic">
                              Or restart manually with: <code class="text-xs">systemctl restart pulse</code>
                            </p>
                          </div>
                        </Show>
                        
                        <Show when={versionInfo()?.deploymentType === 'docker'}>
                          <div class="space-y-1">
                            <p class="text-xs text-gray-700 dark:text-gray-300">Restart your Docker container:</p>
                            <code class="block text-xs bg-gray-100 dark:bg-gray-700 p-2 rounded mt-1">
                              docker restart pulse
                            </code>
                          </div>
                        </Show>
                        
                        <Show when={versionInfo()?.deploymentType === 'systemd' || versionInfo()?.deploymentType === 'manual'}>
                          <div class="space-y-1">
                            <p class="text-xs text-gray-700 dark:text-gray-300">Restart the service:</p>
                            <code class="block text-xs bg-gray-100 dark:bg-gray-700 p-2 rounded mt-1">
                              sudo systemctl restart pulse
                            </code>
                          </div>
                        </Show>
                        
                        <Show when={versionInfo()?.deploymentType === 'development'}>
                          <div class="space-y-1">
                            <p class="text-xs text-gray-700 dark:text-gray-300">Restart the development server:</p>
                            <code class="block text-xs bg-gray-100 dark:bg-gray-700 p-2 rounded mt-1">
                              sudo systemctl restart pulse-backend
                            </code>
                          </div>
                        </Show>
                        
                        <Show when={!versionInfo()?.deploymentType}>
                          <div class="space-y-1">
                            <p class="text-xs text-gray-700 dark:text-gray-300">Restart Pulse using your deployment method</p>
                          </div>
                        </Show>
                      </div>
                      
                      <div class="mt-3 p-2 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded">
                        <p class="text-xs text-green-700 dark:text-green-300">
                          💡 <strong>Tip:</strong> Make sure you've saved your credentials before restarting!
                        </p>
                      </div>
                    </div>
                  </div>
                </div>
              </Show>
              
              {/* Security setup now handled by first-run wizard */}

              {/* API Token - Show always to allow API access even when auth is disabled */}
              <Show when={!securityStatusLoading()}>
                <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
                  {/* Header */}
                  <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                    <div class="flex items-center gap-3">
                      <div class="p-2 bg-blue-100 dark:bg-blue-900/50 rounded-lg">
                        <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
                        </svg>
                      </div>
                      <div>
                        <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-100">API Token</h3>
                        <p class="text-xs text-gray-600 dark:text-gray-400">For automation and integrations</p>
                      </div>
                    </div>
                  </div>
                  
                  {/* Content */}
                  <div class="p-6">
                    {/* Show explanation when auth is disabled */}
                    <Show when={!securityStatus()?.hasAuthentication}>
                      <div class="mb-4 p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
                        <p class="text-xs text-blue-800 dark:text-blue-200">
                          <strong>API Access Control:</strong> Even though authentication is disabled, you can still use API tokens to protect API access for automation and integrations.
                        </p>
                      </div>
                    </Show>
                    <GenerateAPIToken currentTokenHint={securityStatus()?.apiTokenHint} />
                  </div>
                </div>
              </Show>

              {/* Advanced - Only show if auth is enabled */}
              {/* Advanced Options section removed - was only used for Registration Tokens */}
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
                    <button type="button"
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
                            <div>Runtime: {diagnosticsData()?.runtime || 'Unknown'}</div>
                            <div>Memory: {Math.round((diagnosticsData()?.system?.memory?.alloc || 0) / 1024 / 1024)} MB</div>
                          </div>
                        </div>
                        
                        {/* Nodes Status */}
                        <Show when={diagnosticsData()?.nodes && diagnosticsData()!.nodes.length > 0}>
                          <div class="bg-white dark:bg-gray-800 rounded-lg p-3">
                            <h5 class="text-sm font-semibold mb-2 text-gray-700 dark:text-gray-300">PVE Nodes</h5>
                            <For each={diagnosticsData()?.nodes || []}>
                              {(node) => (
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
                        <Show when={diagnosticsData()?.pbs && diagnosticsData()!.pbs.length > 0}>
                          <div class="bg-white dark:bg-gray-800 rounded-lg p-3">
                            <h5 class="text-sm font-semibold mb-2 text-gray-700 dark:text-gray-300">PBS Instances</h5>
                            <For each={diagnosticsData()?.pbs || []}>
                              {(pbs) => (
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
                        <span class="text-gray-600 dark:text-gray-400">Server Port:</span>
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
                    
                    {/* Helper function to sanitize sensitive data */}
                    {(() => {
                      const sanitizeForGitHub = (data: any) => {
                        // Deep clone the data
                        const sanitized = JSON.parse(JSON.stringify(data));
                        
                        // Sanitize IP addresses (keep first octet for network type identification)
                        const sanitizeIP = (ip: string) => {
                          if (!ip) return ip;
                          const parts = ip.split('.');
                          if (parts.length === 4) {
                            return `${parts[0]}.xxx.xxx.xxx`;
                          }
                          return 'xxx.xxx.xxx.xxx';
                        };
                        
                        // Sanitize hostname but keep domain suffix for context
                        const sanitizeHostname = (hostname: string) => {
                          if (!hostname) return hostname;
                          // Keep common suffixes like .lan, .local, .home
                          const suffixMatch = hostname.match(/\.(lan|local|home|internal)$/);
                          const suffix = suffixMatch ? suffixMatch[0] : '';
                          return `node-REDACTED${suffix}`;
                        };
                        
                        // Sanitize nodes
                        if (sanitized.nodes) {
                          sanitized.nodes = sanitized.nodes.map((node: any, index: number) => ({
                            ...node,
                            id: `${node.type}-${index}`,
                            name: sanitizeHostname(node.name),
                            host: node.host ? node.host.replace(/https?:\/\/[^:\/]+/, 'https://REDACTED') : node.host,
                            tokenName: node.tokenName ? 'token-REDACTED' : node.tokenName,
                            clusterName: node.clusterName ? 'cluster-REDACTED' : node.clusterName,
                            clusterEndpoints: node.clusterEndpoints ? node.clusterEndpoints.map((ep: any, epIndex: number) => ({
                              ...ep,
                              NodeName: `node-${epIndex + 1}`,
                              Host: `node-${epIndex + 1}`,
                              IP: sanitizeIP(ep.IP)
                            })) : node.clusterEndpoints
                          }));
                        }
                        
                        // Sanitize websocket URL
                        if (sanitized.websocket?.url) {
                          sanitized.websocket.url = sanitized.websocket.url.replace(/\/\/[^\/]+/, '//REDACTED');
                        }
                        
                        // Add sanitization notice
                        sanitized._notice = 'This diagnostic data has been sanitized for sharing on GitHub. IP addresses, hostnames, and tokens have been redacted.';
                        
                        return sanitized;
                      };
                      
                      const exportDiagnostics = (sanitize: boolean) => {
                        let diagnostics = {
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
                          }
                        };
                        
                        if (sanitize) {
                          diagnostics = sanitizeForGitHub(diagnostics);
                        }
                        
                        const blob = new Blob([JSON.stringify(diagnostics, null, 2)], { type: 'application/json' });
                        const url = URL.createObjectURL(blob);
                        const a = document.createElement('a');
                        a.href = url;
                        const type = sanitize ? 'sanitized' : 'full';
                        a.download = `pulse-diagnostics-${type}-${new Date().toISOString().split('T')[0]}.json`;
                        document.body.appendChild(a);
                        a.click();
                        document.body.removeChild(a);
                        URL.revokeObjectURL(url);
                      };
                      
                      return (
                        <div class="flex gap-2">
                          <button type="button"
                            onClick={() => exportDiagnostics(false)}
                            class="px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                          >
                            Export Full
                          </button>
                          <button type="button"
                            onClick={() => exportDiagnostics(true)}
                            class="px-4 py-2 text-sm bg-green-600 text-white rounded-lg hover:bg-green-700 transition-colors"
                          >
                            Export for GitHub
                          </button>
                        </div>
                      );
                    })()}
                    
                    <p class="text-xs text-gray-500 dark:text-gray-400 mt-3">
                      <strong>Export Full:</strong> Complete data for private troubleshooting<br/>
                      <strong>Export for GitHub:</strong> Sanitized data safe for public sharing
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </Show>
          
          {/* Guest URLs Tab */}
          <Show when={activeTab() === 'urls'}>
            <GuestURLs 
              hasUnsavedChanges={hasUnsavedChanges}
              setHasUnsavedChanges={setHasUnsavedChanges}
            />
          </Show>
        </div>
      </div>
      
      {/* Node Modal - Use separate modals for PVE and PBS to ensure clean state */}
      <Show when={showNodeModal() && currentNodeType() === 'pve'}>
        <NodeModal
          isOpen={true}
          resetKey={modalResetKey()}
          onClose={() => {
            setShowNodeModal(false);
            setEditingNode(null);
            // Increment resetKey to force form reset on next open
            setModalResetKey(prev => prev + 1);
          }}
          nodeType="pve"
          editingNode={editingNode()?.type === 'pve' ? editingNode() ?? undefined : undefined}
          securityStatus={securityStatus() ?? undefined}
          onSave={async (nodeData) => {
          try {
            if (editingNode() && editingNode()!.id) {
              // Update existing node (only if it has a valid ID)
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
      </Show>
      
      {/* PBS Node Modal - Separate instance to prevent contamination */}
      <Show when={showNodeModal() && currentNodeType() === 'pbs'}>
        <NodeModal
          isOpen={true}
          resetKey={modalResetKey()}
          onClose={() => {
            setShowNodeModal(false);
            setEditingNode(null);
            // Increment resetKey to force form reset on next open
            setModalResetKey(prev => prev + 1);
          }}
          nodeType="pbs"
          editingNode={editingNode()?.type === 'pbs' ? editingNode() ?? undefined : undefined}
          securityStatus={securityStatus() ?? undefined}
          onSave={async (nodeData) => {
          try {
            if (editingNode() && editingNode()!.id) {
              // Update existing node (only if it has a valid ID)
              await NodesAPI.updateNode(editingNode()!.id, nodeData as NodeConfig);
              
              // Update local state
              setNodes(nodes().map(n => 
                n.id === editingNode()!.id 
                  ? { 
                      ...n, 
                      ...nodeData,
                      hasPassword: nodeData.password ? true : n.hasPassword,
                      hasToken: nodeData.tokenValue ? true : n.hasToken,
                      status: n.status
                    } 
                  : n
              ));
              showSuccess('Node updated successfully');
            } else {
              // Add new node
              await NodesAPI.addNode(nodeData as NodeConfig);
              
              // Reload the nodes list to get the latest state
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
      </Show>
    </div>
      {/* Export Dialog */}
      <Show when={showExportDialog()}>
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div class="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md w-full">
            <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">Export Configuration</h3>
            
            <div class="space-y-4">
              {/* Password Choice Section - Only show if auth is enabled */}
              <Show when={securityStatus()?.hasAuthentication}>
                <div class="bg-gray-50 dark:bg-gray-900/50 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
                  <div class="space-y-3">
                    <label class="flex items-start gap-3 cursor-pointer">
                      <input
                        type="radio"
                        checked={!useCustomPassphrase()}
                        onChange={() => {
                          setUseCustomPassphrase(false);
                          setExportPassphrase('');
                        }}
                        class="mt-1 text-blue-600 focus:ring-blue-500"
                      />
                      <div class="flex-1">
                        <div class="text-sm font-medium text-gray-700 dark:text-gray-300">
                          Use your login password
                        </div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                          Use the same password you use to log into Pulse (recommended)
                        </div>
                      </div>
                    </label>
                    
                    <label class="flex items-start gap-3 cursor-pointer">
                      <input
                        type="radio"
                        checked={useCustomPassphrase()}
                        onChange={() => setUseCustomPassphrase(true)}
                        class="mt-1 text-blue-600 focus:ring-blue-500"
                      />
                      <div class="flex-1">
                        <div class="text-sm font-medium text-gray-700 dark:text-gray-300">
                          Use a custom passphrase
                        </div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                          Create a different passphrase for this backup
                        </div>
                      </div>
                    </label>
                  </div>
                </div>
              </Show>
              
              {/* Show password input based on selection */}
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  {securityStatus()?.hasAuthentication 
                    ? (useCustomPassphrase() ? 'Custom Passphrase' : 'Enter Your Login Password')
                    : 'Encryption Passphrase'}
                </label>
                <input
                  type="password"
                  value={exportPassphrase()}
                  onInput={(e) => setExportPassphrase(e.currentTarget.value)}
                  placeholder={
                    securityStatus()?.hasAuthentication 
                      ? (useCustomPassphrase() ? "Enter a strong passphrase" : "Enter your Pulse login password")
                      : "Enter a strong passphrase for encryption"
                  }
                  class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                />
                <Show when={!securityStatus()?.hasAuthentication || useCustomPassphrase()}>
                  <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    You'll need this passphrase to restore the backup.
                  </p>
                </Show>
                <Show when={securityStatus()?.hasAuthentication && !useCustomPassphrase()}>
                  <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    You'll use this same password when restoring the backup
                  </p>
                </Show>
              </div>
              
              <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-3">
                <div class="flex gap-2">
                  <svg class="w-4 h-4 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                  </svg>
                  <div class="text-xs text-amber-700 dark:text-amber-300">
                    <strong>Important:</strong> The backup contains node credentials but NOT authentication settings. 
                    Each Pulse instance should configure its own login credentials for security.
                    Remember your {useCustomPassphrase() || !securityStatus()?.hasAuthentication ? 'passphrase' : 'password'} for restoring.
                  </div>
                </div>
              </div>
              
              <div class="flex justify-end space-x-3">
                <button type="button"
                  onClick={() => {
                    setShowExportDialog(false);
                    setExportPassphrase('');
                    setUseCustomPassphrase(false);
                  }}
                  class="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
                >
                  Cancel
                </button>
                <button type="button"
                  onClick={handleExport}
                  disabled={!exportPassphrase() || (useCustomPassphrase() && exportPassphrase().length < 12)}
                  class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Export
                </button>
              </div>
            </div>
          </div>
        </div>
      </Show>
      
      {/* API Token Modal */}
      <Show when={showApiTokenModal()}>
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div class="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md w-full">
            <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">
              API Token Required
            </h3>
            
            <div class="space-y-4">
              <p class="text-sm text-gray-600 dark:text-gray-400">
                This Pulse instance requires an API token for export/import operations. Please enter the API token configured on the server.
              </p>
              
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  API Token
                </label>
                <input
                  type="password"
                  value={apiTokenInput()}
                  onInput={(e) => setApiTokenInput(e.currentTarget.value)}
                  placeholder="Enter API token"
                  class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-gray-200"
                />
              </div>
              
              <div class="text-xs text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-700 rounded p-2">
                <p class="font-semibold mb-1">The API token is set as an environment variable:</p>
                <code class="block">API_TOKEN=your-secure-token</code>
              </div>
            </div>
            
            <div class="flex justify-end space-x-2 mt-6">
              <button type="button"
                onClick={() => {
                  setShowApiTokenModal(false);
                  setApiTokenInput('');
                }}
                class="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
              >
                Cancel
              </button>
              <button type="button"
                onClick={() => {
                  if (apiTokenInput()) {
                    localStorage.setItem('apiToken', apiTokenInput());
                    const source = apiTokenModalSource();
                    setShowApiTokenModal(false);
                    setApiTokenInput('');
                    setApiTokenModalSource(null);
                    // Retry the operation that triggered the modal
                    if (source === 'export') {
                      handleExport();
                    } else if (source === 'import') {
                      handleImport();
                    }
                  } else {
                    showError('Please enter the API token');
                  }
                }}
                disabled={!apiTokenInput()}
                class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Authenticate
              </button>
            </div>
          </div>
        </div>
      </Show>
      
      {/* Import Dialog */}
      <Show when={showImportDialog()}>
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div class="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md w-full">
            <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">Import Configuration</h3>
            
            <div class="space-y-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Configuration File
                </label>
                <input
                  type="file"
                  accept=".json"
                  onChange={(e) => {
                    const file = e.currentTarget.files?.[0];
                    if (file) setImportFile(file);
                  }}
                  class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                />
              </div>
              
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Backup Password
                </label>
                <input
                  type="password"
                  value={importPassphrase()}
                  onInput={(e) => setImportPassphrase(e.currentTarget.value)}
                  placeholder="Enter the password used when creating this backup"
                  class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                />
                <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  This is usually your Pulse login password, unless you used a custom passphrase
                </p>
              </div>
              
              <div class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded p-3">
                <p class="text-xs text-yellow-700 dark:text-yellow-300">
                  <strong>Warning:</strong> Importing will replace all current configuration. This action cannot be undone.
                </p>
              </div>
              
              <div class="flex justify-end space-x-3">
                <button type="button"
                  onClick={() => {
                    setShowImportDialog(false);
                    setImportPassphrase('');
                    setImportFile(null);
                  }}
                  class="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
                >
                  Cancel
                </button>
                <button type="button"
                  onClick={handleImport}
                  disabled={!importPassphrase() || !importFile()}
                  class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Import
                </button>
              </div>
            </div>
          </div>
        </div>
      </Show>
      
      <ChangePasswordModal
        isOpen={showPasswordModal()}
        onClose={() => {
          setShowPasswordModal(false);
          // Refresh security status after password change
          loadSecurityStatus();
        }}
      />
    </>
  );
};

export default Settings;