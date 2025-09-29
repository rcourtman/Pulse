import { Component, createSignal, onMount, For, Show, createEffect, onCleanup } from 'solid-js';
import { useWebSocket } from '@/App';
import { showSuccess, showError } from '@/utils/toast';
import { NodeModal } from './NodeModal';
import { GenerateAPIToken } from './GenerateAPIToken';
import { ChangePasswordModal } from './ChangePasswordModal';
import { GuestURLs } from './GuestURLs';
import { OIDCPanel } from './OIDCPanel';
import { QuickSecuritySetup } from './QuickSecuritySetup';
import { SecurityPostureSummary } from './SecurityPostureSummary';
import { SettingsAPI } from '@/api/settings';
import { NodesAPI } from '@/api/nodes';
import { UpdatesAPI } from '@/api/updates';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { Toggle } from '@/components/shared/Toggle';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';
import type { NodeConfig } from '@/types/nodes';
import type { UpdateInfo, VersionInfo } from '@/api/updates';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';
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

type RawDiscoveredServer = {
  ip?: string;
  port?: number;
  type?: string;
  version?: string;
  hostname?: string;
  name?: string;
  release?: string;
};

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

interface DiscoveryScanStatus {
  scanning: boolean;
  subnet?: string;
  lastScanStartedAt?: number;
  lastResultAt?: number;
  errors?: string[];
}

type SettingsTab = 'pve' | 'pbs' | 'system' | 'urls' | 'security' | 'diagnostics';

// Node with UI-specific fields
type NodeConfigWithStatus = NodeConfig & {
  hasPassword?: boolean;
  hasToken?: boolean;
  status: 'connected' | 'disconnected' | 'error' | 'pending';
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
  const [discoveryScanStatus, setDiscoveryScanStatus] = createSignal<DiscoveryScanStatus>({ scanning: false });
  
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
  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatusInfo | null>(null);
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
  const [showQuickSecuritySetup, setShowQuickSecuritySetup] = createSignal(false);
  const [showQuickSecurityWizard, setShowQuickSecurityWizard] = createSignal(false);

  const formatTimestamp = (timestamp?: string) => {
    if (!timestamp) {
      return 'Unknown';
    }

    const date = new Date(timestamp);
    if (Number.isNaN(date.getTime())) {
      return 'Unknown';
    }

    return date.toLocaleString();
  };

  const formatRelativeTime = (timestamp?: number) => {
    if (!timestamp) {
      return '';
    }

    const delta = Date.now() - timestamp;
    if (delta < 0) {
      return 'just now';
    }

    const seconds = Math.round(delta / 1000);
    if (seconds < 60) {
      return `${seconds}s ago`;
    }

    const minutes = Math.round(seconds / 60);
    if (minutes < 60) {
      return `${minutes}m ago`;
    }

    const hours = Math.round(minutes / 60);
    if (hours < 24) {
      return `${hours}h ago`;
    }

    return new Date(timestamp).toLocaleString();
  };

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
        status: node.status || 'pending' as const
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
        console.log('Security status loaded:', status);
        setSecurityStatus(status);
      } else {
        console.error('Failed to fetch security status:', response.status);
      }
    } catch (err) {
      console.error('Failed to fetch security status:', err);
    } finally {
      setSecurityStatusLoading(false);
    }
  };

  const updateDiscoveredNodesFromServers = (servers: RawDiscoveredServer[] | undefined | null, options: { merge?: boolean } = {}) => {
    const { merge = false } = options;

    if (!servers || servers.length === 0) {
      if (!merge) {
        setDiscoveredNodes([]);
      }
      return;
    }

    // Prepare sets of configured hosts and cluster member IPs to filter duplicates
    const configuredHosts = new Set<string>();
    const clusterMemberIPs = new Set<string>();

    nodes().forEach((n) => {
      const cleanedHost = n.host.replace(/^https?:\/\//, '').replace(/:\d+$/, '');
      configuredHosts.add(cleanedHost.toLowerCase());

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

    const normalized = servers
      .map((server): DiscoveredServer | null => {
        const ip = (server.ip || '').trim();
        const type = (server.type || '').toLowerCase();
        const port = typeof server.port === 'number' ? server.port : type === 'pbs' ? 8007 : 8006;

        if (!ip || (type !== 'pve' && type !== 'pbs')) {
          return null;
        }

        const hostname = (server.hostname || server.name || '').trim();
        const version = (server.version || '').trim();
        const release = (server.release || '').trim();

        return {
          ip,
          port,
          type: type as 'pve' | 'pbs',
          version: version || 'Unknown',
          hostname: hostname || undefined,
          release: release || undefined,
        };
      })
      .filter((server): server is DiscoveredServer => server !== null);

    const filtered = normalized.filter((server) => {
      const serverIP = server.ip.toLowerCase();
      const serverHostname = server.hostname?.toLowerCase();

      if (configuredHosts.has(serverIP) || (serverHostname && configuredHosts.has(serverHostname))) {
        return false;
      }

      if (clusterMemberIPs.has(serverIP) || (serverHostname && clusterMemberIPs.has(serverHostname))) {
        return false;
      }

      return true;
    });

    if (merge) {
      setDiscoveredNodes((prev) => {
        const existingMap = new Map(prev.map((item) => [`${item.ip}:${item.port}`, item]));
        filtered.forEach((server) => {
          existingMap.set(`${server.ip}:${server.port}`, server);
        });
        return Array.from(existingMap.values());
      });
    } else {
      setDiscoveredNodes(filtered);
    }

    setDiscoveryScanStatus(prev => ({
      ...prev,
      lastResultAt: Date.now()
    }));
  };

  const loadDiscoveredNodes = async () => {
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/discover');
      if (response.ok) {
        const data = await response.json();
        if (Array.isArray(data.servers)) {
          updateDiscoveredNodesFromServers(data.servers as RawDiscoveredServer[]);
          setDiscoveryScanStatus(prev => ({
            ...prev,
            lastResultAt: typeof data.timestamp === 'number' ? data.timestamp : Date.now(),
            errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined
          }));
        } else {
          updateDiscoveredNodesFromServers([]);
          setDiscoveryScanStatus(prev => ({
            ...prev,
            lastResultAt: typeof data?.timestamp === 'number' ? data.timestamp : prev.lastResultAt,
            errors: Array.isArray(data?.errors) && data.errors.length > 0 ? data.errors : undefined
          }));
        }
      }
    } catch (error) {
      console.error('Failed to load discovered nodes:', error);
    }
  };

  const triggerDiscoveryScan = async (options: { quiet?: boolean } = {}) => {
    const { quiet = false } = options;

    setDiscoveryScanStatus(prev => ({
      ...prev,
      scanning: true,
      subnet: discoverySubnet() || prev.subnet,
      lastScanStartedAt: Date.now(),
      errors: undefined
    }));

    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/discover', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ subnet: discoverySubnet() || 'auto' })
      });

      if (!response.ok) {
        const message = await response.text();
        throw new Error(message || 'Discovery request failed');
      }

      if (!quiet) {
        notificationStore.info('Discovery scan started', 2000);
      }
    } catch (error) {
      console.error('Failed to start discovery scan:', error);
      notificationStore.error('Failed to start discovery scan');
      setDiscoveryScanStatus(prev => ({
        ...prev,
        scanning: false
      }));
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
      if (!data) {
        updateDiscoveredNodesFromServers([]);
        setDiscoveryScanStatus(prev => ({
          ...prev,
          scanning: false
        }));
        return;
      }

      if (Array.isArray(data.servers)) {
        updateDiscoveredNodesFromServers(data.servers as RawDiscoveredServer[], { merge: !!data.immediate });
        setDiscoveryScanStatus(prev => ({
          ...prev,
          scanning: data.scanning ?? prev.scanning,
          lastResultAt: data.timestamp ?? Date.now(),
          errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined
        }));
      } else if (!data.immediate) {
        // Ensure we clear stale results when the update explicitly reports no servers
        updateDiscoveredNodesFromServers([]);
        setDiscoveryScanStatus(prev => ({
          ...prev,
          scanning: data.scanning ?? prev.scanning,
          lastResultAt: data.timestamp ?? prev.lastResultAt,
          errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined
        }));
      } else {
        setDiscoveryScanStatus(prev => ({
          ...prev,
          scanning: data.scanning ?? prev.scanning,
          errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined
        }));
      }
    });

    const unsubscribeDiscoveryStatus = eventBus.on('discovery_status', (data) => {
      if (!data) {
        setDiscoveryScanStatus(prev => ({
          ...prev,
          scanning: false
        }));
        return;
      }

      setDiscoveryScanStatus(prev => ({
        ...prev,
        scanning: !!data.scanning,
        subnet: data.subnet || prev.subnet,
        lastScanStartedAt: data.scanning
          ? (data.timestamp ?? Date.now())
          : prev.lastScanStartedAt,
        lastResultAt: !data.scanning && data.timestamp
          ? data.timestamp
          : prev.lastResultAt
      }));
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
      unsubscribeDiscoveryStatus();
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
      <Card padding="md">
        <SectionHeader
          title="Configuration settings"
          description="Manage Proxmox nodes and system configuration"
          size="lg"
        />
      </Card>
      
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
      <Card padding="none">
        <div class="p-1">
          <div class="flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5 w-full overflow-x-auto scrollbar-hide" style="-webkit-overflow-scrolling: touch;">
            <For each={tabs}>
              {(tab) => (
                <button type="button"
                  class={`flex-1 px-2 sm:px-3 py-1.5 sm:py-2 text-xs sm:text-sm font-medium rounded-md transition-all whitespace-nowrap ${
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
              <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between mb-4 gap-3">
                <SectionHeader title="Proxmox VE nodes" size="md" class="flex-1" />
                <div class="flex flex-wrap gap-2 items-center justify-end">
                  {/* Discovery toggle */}
                  <div class="flex items-center gap-2 sm:gap-3" title="Enable automatic discovery of Proxmox servers on your network">
                    <span class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">Discovery</span>
                    <Toggle
                      checked={discoveryEnabled()}
                      onChange={async (e) => {
                        if (envOverrides().discoveryEnabled) {
                          return;
                        }
                        const newValue = e.currentTarget.checked;
                        setDiscoveryEnabled(newValue);
                        try {
                          await SettingsAPI.updateSystemSettings({
                            discoveryEnabled: newValue,
                            discoverySubnet: discoverySubnet()
                          });
                          if (newValue) {
                            await triggerDiscoveryScan({ quiet: true });
                            notificationStore.success('Discovery enabled — scanning network...', 2000);
                          } else {
                            notificationStore.info('Discovery disabled', 2000);
                            setDiscoveryScanStatus(prev => ({
                              ...prev,
                              scanning: false
                            }));
                          }
                        } catch (error) {
                          console.error('Failed to update discovery setting:', error);
                          notificationStore.error('Failed to update discovery setting');
                          setDiscoveryEnabled(!newValue);
                        } finally {
                          await loadDiscoveredNodes();
                        }
                      }}
                      disabled={envOverrides().discoveryEnabled}
                      containerClass="gap-2"
                      label={<span class="text-xs font-medium text-gray-600 dark:text-gray-400">{discoveryEnabled() ? 'On' : 'Off'}</span>}
                    />
                  </div>
                  
                  <Show when={discoveryEnabled()}>
                    <button type="button" 
                      onClick={async () => {
                        notificationStore.info('Refreshing discovery...', 2000);
                        try {
                          await triggerDiscoveryScan({ quiet: true });
                        } finally {
                          await loadDiscoveredNodes();
                        }
                      }}
                      class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors flex items-center gap-1"
                      title="Refresh discovered servers"
                    >
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="23 4 23 10 17 10"></polyline>
                        <polyline points="1 20 1 14 7 14"></polyline>
                        <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"></path>
                      </svg>
                      <span class="hidden sm:inline">Refresh</span>
                    </button>
                  </Show>
                  
                  <button type="button" 
                    onClick={() => {
                      setEditingNode(null);
                      setCurrentNodeType('pve');
                      setModalResetKey(prev => prev + 1);
                      setShowNodeModal(true);
                    }}
                    class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors flex items-center gap-1"
                  >
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <line x1="12" y1="5" x2="12" y2="19"></line>
                      <line x1="5" y1="12" x2="19" y2="12"></line>
                    </svg>
                    <span class="sm:hidden">Add</span>
                    <span class="hidden sm:inline">Add PVE Node</span>
                  </button>
                </div>
              </div>
              
              <div class="grid gap-4">
                <For each={nodes().filter(n => n.type === 'pve')}>
                  {(node) => (
                    <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4 border border-gray-200 dark:border-gray-600">
                      <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-2">
                        <div class="flex-1 min-w-0">
                          <div class="flex items-start gap-3">
                            <div class={`flex-shrink-0 w-3 h-3 mt-1.5 rounded-full ${
                              (() => {
                                // Find the corresponding node in the WebSocket state
                                const stateNode = state.nodes.find(n => n.instance === node.name);
                                // Check if the node has an unhealthy connection or is offline
                                if (stateNode?.connectionHealth === 'unhealthy' || stateNode?.connectionHealth === 'error' || stateNode?.status === 'offline') {
                                  return 'bg-red-500';
                                }
                                // Check if connection is degraded (partial cluster connectivity)
                                if (stateNode?.connectionHealth === 'degraded') {
                                  return 'bg-yellow-500';
                                }
                                // Check if we have a healthy connection
                                if (stateNode && (stateNode.status === 'online' || stateNode.connectionHealth === 'healthy')) {
                                  return 'bg-green-500';
                                }
                                // Fall back to the last known config status if live data hasn't arrived yet
                                if (node.status === 'connected') {
                                  return 'bg-green-500';
                                }
                                if (node.status === 'error') {
                                  return 'bg-red-500';
                                }
                                if (node.status === 'pending' || node.status === 'disconnected') {
                                  return 'bg-amber-500 animate-pulse';
                                }
                                return 'bg-gray-400';
                              })()
                            }`}></div>
                            <div class="flex-1 min-w-0">
                              <h4 class="font-medium text-gray-900 dark:text-gray-100 truncate">{node.name}</h4>
                              <p class="text-sm text-gray-600 dark:text-gray-400 mt-1 break-all">{node.host}</p>
                              <div class="flex flex-wrap gap-1 sm:gap-2 mt-2">
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
                        </div>
                        <div class="flex items-center gap-1 sm:gap-2 flex-shrink-0">
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
                  <div class="space-y-3">
                    <div class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-400">
                      <Show when={discoveryScanStatus().scanning}>
                        <span class="flex items-center gap-2">
                          <svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <circle cx="12" cy="12" r="10" stroke-opacity="0.25"></circle>
                            <path d="M22 12a10 10 0 00-10-10" stroke-linecap="round"></path>
                          </svg>
                          <span>Scanning your network for Proxmox VE servers…</span>
                        </span>
                      </Show>
                      <Show when={!discoveryScanStatus().scanning && (discoveryScanStatus().lastResultAt || discoveryScanStatus().lastScanStartedAt)}>
                        <span>
                          Last scan {formatRelativeTime(discoveryScanStatus().lastResultAt ?? discoveryScanStatus().lastScanStartedAt)}
                        </span>
                      </Show>
                    </div>
                    <Show when={discoveryScanStatus().errors && discoveryScanStatus().errors!.length}>
                      <div class="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-2">
                        <span class="font-medium">Discovery issues:</span>
                        <ul class="list-disc ml-4 mt-1 space-y-0.5">
                          <For each={discoveryScanStatus().errors || []}>
                            {(err) => <li>{err}</li>}
                          </For>
                        </ul>
                      </div>
                    </Show>
                    <Show when={discoveryScanStatus().scanning && discoveredNodes().filter(n => n.type === 'pve').length === 0}>
                      <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                        <svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <circle cx="12" cy="12" r="10" stroke-opacity="0.25"></circle>
                          <path d="M22 12a10 10 0 00-10-10" stroke-linecap="round"></path>
                        </svg>
                        <span>Waiting for responses… this can take up to a minute depending on your network size.</span>
                      </div>
                    </Show>
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
                              user: '',
                              tokenName: '',
                              tokenValue: '',
                              verifySSL: false,
                              monitorVMs: true,
                              monitorContainers: true,
                              monitorStorage: true,
                              monitorBackups: true,
                              status: 'pending'
                            } as NodeConfigWithStatus);
                            setCurrentNodeType('pve');
                            setShowNodeModal(true);
                          }}
                        >
                          <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-2">
                            <div class="flex-1 min-w-0">
                              <div class="flex items-start gap-3">
                                <div class="flex-shrink-0 w-3 h-3 mt-1.5 rounded-full bg-gray-400 animate-pulse"></div>
                                <div class="flex-1 min-w-0">
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
                            </div>
                            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" class="text-gray-400 mt-1">
                              <path d="M12 5v14m-7-7h14" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                            </svg>
                          </div>
                        </div>
                      )}
                    </For>
                  </div>
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
              <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between mb-4 gap-3">
                <SectionHeader title="Proxmox Backup Server nodes" size="md" class="flex-1" />
                <div class="flex flex-wrap gap-2 items-center justify-end">
                  {/* Discovery toggle */}
                  <div class="flex items-center gap-2" title="Enable automatic discovery of PBS servers on your network">
                    <span class="text-sm text-gray-600 dark:text-gray-400">Discovery</span>
                    <Toggle
                      checked={discoveryEnabled()}
                      onChange={async (e) => {
                        if (envOverrides().discoveryEnabled) {
                          return;
                        }
                        const newValue = e.currentTarget.checked;
                        setDiscoveryEnabled(newValue);
                        try {
                          await SettingsAPI.updateSystemSettings({
                            discoveryEnabled: newValue,
                            discoverySubnet: discoverySubnet()
                          });
                          if (newValue) {
                            await triggerDiscoveryScan({ quiet: true });
                            notificationStore.success('Discovery enabled — scanning network...', 2000);
                          } else {
                            notificationStore.info('Discovery disabled', 2000);
                            setDiscoveryScanStatus(prev => ({
                              ...prev,
                              scanning: false
                            }));
                          }
                        } catch (error) {
                          console.error('Failed to update discovery setting:', error);
                          notificationStore.error('Failed to update discovery setting');
                          setDiscoveryEnabled(!newValue);
                        } finally {
                          await loadDiscoveredNodes();
                        }
                      }}
                      disabled={envOverrides().discoveryEnabled}
                      containerClass="gap-2"
                      label={<span class="text-xs font-medium text-gray-600 dark:text-gray-400">{discoveryEnabled() ? 'On' : 'Off'}</span>}
                    />
                  </div>
                  
                  <Show when={discoveryEnabled()}>
                    <button type="button" 
                      onClick={async () => {
                        notificationStore.info('Refreshing discovery...', 2000);
                        try {
                          await triggerDiscoveryScan({ quiet: true });
                        } finally {
                          await loadDiscoveredNodes();
                        }
                      }}
                      class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors flex items-center gap-1"
                      title="Refresh discovered servers"
                    >
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="23 4 23 10 17 10"></polyline>
                        <polyline points="1 20 1 14 7 14"></polyline>
                        <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"></path>
                      </svg>
                      <span class="hidden sm:inline">Refresh</span>
                    </button>
                  </Show>
                  
                  <button type="button" 
                    onClick={() => {
                      setEditingNode(null);
                      setCurrentNodeType('pbs');
                      setModalResetKey(prev => prev + 1);
                      setShowNodeModal(true);
                    }}
                    class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors flex items-center gap-1"
                  >
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <line x1="12" y1="5" x2="12" y2="19"></line>
                      <line x1="5" y1="12" x2="19" y2="12"></line>
                    </svg>
                    <span class="sm:hidden">Add</span>
                    <span class="hidden sm:inline">Add PBS Node</span>
                  </button>
                </div>
              </div>
              
              <div class="grid gap-4">
                <For each={nodes().filter(n => n.type === 'pbs')}>
                  {(node) => (
                    <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4 border border-gray-200 dark:border-gray-600">
                      <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-2">
                        <div class="flex-1 min-w-0">
                          <div class="flex items-start gap-3">
                            <div class={`flex-shrink-0 w-3 h-3 mt-1.5 rounded-full ${
                              (() => {
                                // Find the corresponding PBS instance in the WebSocket state
                                const statePBS = state.pbs.find(p => p.name === node.name);
                                // Check if the PBS has an unhealthy connection or is offline
                                if (statePBS?.connectionHealth === 'unhealthy' || statePBS?.connectionHealth === 'error' || statePBS?.status === 'offline') {
                                  return 'bg-red-500';
                                }
                                // Check if connection is degraded (not commonly used for PBS but keeping consistent)
                                if (statePBS?.connectionHealth === 'degraded') {
                                  return 'bg-yellow-500';
                                }
                                // Check if we have a healthy connection
                                if (statePBS && (statePBS.status === 'online' || statePBS.connectionHealth === 'healthy')) {
                                  return 'bg-green-500';
                                }
                                // Fall back to the last known config status if live data hasn't arrived yet
                                if (node.status === 'connected') {
                                  return 'bg-green-500';
                                }
                                if (node.status === 'error') {
                                  return 'bg-red-500';
                                }
                                if (node.status === 'pending' || node.status === 'disconnected') {
                                  return 'bg-amber-500 animate-pulse';
                                }
                                return 'bg-gray-400';
                              })()
                            }`}></div>
                            <div class="flex-1 min-w-0">
                              <h4 class="font-medium text-gray-900 dark:text-gray-100 truncate">{node.name}</h4>
                              <p class="text-sm text-gray-600 dark:text-gray-400 mt-1 break-all">{node.host}</p>
                              <div class="flex flex-wrap gap-1 sm:gap-2 mt-2">
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
                        </div>
                        <div class="flex items-center gap-1 sm:gap-2 flex-shrink-0">
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
                  <div class="space-y-3">
                    <div class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-400">
                      <Show when={discoveryScanStatus().scanning}>
                        <span class="flex items-center gap-2">
                          <svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <circle cx="12" cy="12" r="10" stroke-opacity="0.25"></circle>
                            <path d="M22 12a10 10 0 00-10-10" stroke-linecap="round"></path>
                          </svg>
                          <span>Scanning your network for Proxmox Backup Servers…</span>
                        </span>
                      </Show>
                      <Show when={!discoveryScanStatus().scanning && (discoveryScanStatus().lastResultAt || discoveryScanStatus().lastScanStartedAt)}>
                        <span>
                          Last scan {formatRelativeTime(discoveryScanStatus().lastResultAt ?? discoveryScanStatus().lastScanStartedAt)}
                        </span>
                      </Show>
                    </div>
                    <Show when={discoveryScanStatus().errors && discoveryScanStatus().errors!.length}>
                      <div class="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-2">
                        <span class="font-medium">Discovery issues:</span>
                        <ul class="list-disc ml-4 mt-1 space-y-0.5">
                          <For each={discoveryScanStatus().errors || []}>
                            {(err) => <li>{err}</li>}
                          </For>
                        </ul>
                      </div>
                    </Show>
                    <Show when={discoveryScanStatus().scanning && discoveredNodes().filter(n => n.type === 'pbs').length === 0}>
                      <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                        <svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <circle cx="12" cy="12" r="10" stroke-opacity="0.25"></circle>
                          <path d="M22 12a10 10 0 00-10-10" stroke-linecap="round"></path>
                        </svg>
                        <span>Waiting for responses… this can take up to a minute depending on your network size.</span>
                      </div>
                    </Show>
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
                              user: '',
                              tokenName: '',
                              tokenValue: '',
                              verifySSL: false,
                              monitorDatastores: true,
                              monitorSyncJobs: true,
                              monitorVerifyJobs: true,
                              monitorPruneJobs: true,
                              monitorGarbageJobs: true,
                              status: 'pending'
                            } as NodeConfigWithStatus);
                            setCurrentNodeType('pbs');
                            setShowNodeModal(true);
                          }}
                        >
                          <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-2">
                            <div class="flex-1 min-w-0">
                              <div class="flex items-start gap-3">
                                <div class="flex-shrink-0 w-3 h-3 mt-1.5 rounded-full bg-gray-400 animate-pulse"></div>
                                <div class="flex-1 min-w-0">
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
                            </div>
                            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" class="text-gray-400 mt-1">
                              <path d="M12 5v14m-7-7h14" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                            </svg>
                          </div>
                        </div>
                      )}
                    </For>
                  </div>
                </Show>
              </div>
              </Show>
            </div>
          </Show>
          
          {/* System Settings Tab */}
          <Show when={activeTab() === 'system'}>
            <div class="space-y-6">
              <SectionHeader title="System configuration" size="md" class="mb-2" />

              <Card
                tone="info"
                padding="md"
                border={false}
                class="border border-blue-200 dark:border-blue-800"
              >
                <div class="flex items-start gap-3">
                  <svg
                    class="w-5 h-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <div class="text-sm text-blue-800 dark:text-blue-200">
                    <p class="font-medium mb-1">Configuration Priority</p>
                    <ul class="space-y-1">
                      <li>• Some env vars override settings (API_TOKEN, PORTS, AUTH)</li>
                      <li>• Changes made here are saved to system.json immediately</li>
                      <li>• Settings persist unless overridden by env vars</li>
                    </ul>
                  </div>
                </div>
              </Card>

              <div class="grid gap-4 lg:grid-cols-5">
                <Card padding="lg" class="space-y-6 lg:col-span-3">
                  <section class="space-y-3">
                    <h4 class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <circle cx="12" cy="12" r="10"></circle>
                        <path d="M2 12h20M12 2a15.3 15.3 0 014 10 15.3 15.3 0 01-4 10 15.3 15.3 0 01-4-10 15.3 15.3 0 014-10z"></path>
                      </svg>
                      Network Settings
                    </h4>
                    <div class="space-y-2">
                      <label class="text-sm font-medium text-gray-900 dark:text-gray-100">CORS Allowed Origins</label>
                      <p class="text-xs text-gray-600 dark:text-gray-400">For reverse proxy setups (* = allow all, empty = same-origin only)</p>
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
                  </section>

                  <section class="space-y-3">
                    <h4 class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <rect x="3" y="4" width="18" height="14" rx="2"></rect>
                        <path d="M7 20h10"></path>
                      </svg>
                      Embedding
                    </h4>
                    <p class="text-xs text-gray-600 dark:text-gray-400">Allow Pulse to be embedded in iframes (e.g., Homepage dashboard)</p>
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
                        <div class="space-y-2">
                          <label class="text-xs font-medium text-gray-700 dark:text-gray-300">Allowed Embed Origins (optional)</label>
                          <p class="text-xs text-gray-600 dark:text-gray-400">Comma-separated list of origins that can embed Pulse (leave empty for same-origin only)</p>
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
                          <p class="text-xs text-gray-500 dark:text-gray-400">
                            Example: If Pulse is at <code>pulse.my.domain</code> and your dashboard is at <code>my.domain</code>, add <code>https://my.domain</code> here.
                          </p>
                        </div>
                      </Show>
                    </div>
                  </section>

                  <Card tone="warning" padding="sm" border={false} class="border border-amber-200 dark:border-amber-800">
                    <p class="text-xs text-amber-800 dark:text-amber-200 mb-2">
                      <strong>Port Configuration:</strong> Use <code class="font-mono bg-amber-100 dark:bg-amber-800 px-1 rounded">systemctl edit pulse</code>
                    </p>
                    <p class="text-xs text-amber-700 dark:text-amber-300 font-mono">
                      [Service]<br/>
                      Environment="FRONTEND_PORT=8080"<br/>
                      <span class="text-xs text-amber-600 dark:text-amber-400">Then restart: sudo systemctl restart pulse</span>
                    </p>
                  </Card>
                </Card>

                <Card padding="lg" class="space-y-6 lg:col-span-2">
                  <section class="space-y-4">
                    <h4 class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="23 4 23 10 17 10"></polyline>
                        <polyline points="1 20 1 14 7 14"></polyline>
                        <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"></path>
                      </svg>
                      Updates
                    </h4>

                    <div class="space-y-4">
                      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
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

                      <Show when={versionInfo()?.isDocker && !updateInfo()?.available}>
                        <div class="p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
                          <p class="text-xs text-blue-800 dark:text-blue-200">
                            <strong>Docker Installation:</strong> Updates are managed through Docker. Pull the latest image to update.
                          </p>
                        </div>
                      </Show>

                      <Show when={updateInfo()?.available}>
                        <div class="p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg space-y-3">
                          <div>
                            <p class="text-sm font-medium text-green-800 dark:text-green-200">
                              Update Available: {updateInfo()?.latestVersion}
                            </p>
                            <p class="text-xs text-green-700 dark:text-green-300 mt-1">
                              Released: {updateInfo()?.releaseDate ? new Date(updateInfo()!.releaseDate).toLocaleDateString() : 'Unknown'}
                            </p>
                          </div>

                          <div class="p-2 bg-green-100 dark:bg-green-900/40 rounded space-y-2">
                            <p class="text-xs font-medium text-green-800 dark:text-green-200">How to update:</p>
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
                            <Show when={!versionInfo()?.deploymentType && versionInfo()?.isDocker}>
                              <p class="text-xs text-green-700 dark:text-green-300">
                                Pull the latest Pulse Docker image and recreate your container.
                              </p>
                            </Show>
                          </div>

                          <Show when={updateInfo()?.releaseNotes}>
                            <details class="mt-1">
                              <summary class="text-xs text-green-700 dark:text-green-300 cursor-pointer">Release Notes</summary>
                              <pre class="mt-2 text-xs text-green-600 dark:text-green-400 whitespace-pre-wrap font-mono bg-green-100 dark:bg-green-900/30 p-2 rounded">
                                {updateInfo()?.releaseNotes}
                              </pre>
                            </details>
                          </Show>
                        </div>
                      </Show>

                      <div class="border-t border-gray-200 dark:border-gray-600 pt-4 space-y-4">
                        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
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

                        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                          <div>
                            <label class="text-sm font-medium text-gray-900 dark:text-gray-100">Update Checks</label>
                            <p class="text-xs text-gray-600 dark:text-gray-400">
                              Automatically check for updates (installation is manual)
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
                          <div class="space-y-4 rounded-md border border-gray-200 dark:border-gray-600 p-3">
                            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
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

                            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                              <div>
                                <label class="text-sm font-medium text-gray-900 dark:text-gray-100">Check Time</label>
                                <p class="text-xs text-gray-600 dark:text-gray-400">
                                  Preferred time to check for updates
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
                  </section>
                </Card>
              </div>

              {/* Backup & Restore - Moved from Security tab */}
              <Card padding="lg" border={false} class="border border-gray-200 dark:border-gray-700">
                <SectionHeader
                  title="Backup & restore"
                  description="Backup your node configurations and credentials or restore from a previous backup."
                  size="md"
                  class="mb-4"
                />
                
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
              </Card>
            </div>
          </Show>
          
          {/* Security Tab */}
          <Show when={activeTab() === 'security'}>
            <div class="space-y-6">
              <Show when={!securityStatusLoading() && securityStatus()}>
                <SecurityPostureSummary status={securityStatus()!} />
              </Show>

              <Show when={!securityStatusLoading() && securityStatus()?.hasProxyAuth}>
                <Card padding="sm" class="border border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/20">
                  <div class="flex flex-col gap-2 text-xs text-blue-800 dark:text-blue-200">
                    <div class="flex items-center gap-2">
                      <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                      </svg>
                      <span class="font-semibold text-blue-900 dark:text-blue-100">Proxy authentication detected</span>
                    </div>
                    <p>
                      Requests are validated by an upstream proxy. The current proxied user is
                      {securityStatus()?.proxyAuthUsername ? ` ${securityStatus()?.proxyAuthUsername}` : ' available once a request is received'}.
                      {securityStatus()?.proxyAuthIsAdmin ? ' Admin privileges confirmed.' : ''}
                      <Show when={securityStatus()?.proxyAuthLogoutURL}>
                        {' '}
                        <a
                          class="underline font-medium"
                          href={securityStatus()?.proxyAuthLogoutURL}
                        >
                          Proxy logout
                        </a>
                      </Show>
                    </p>
                    <p>
                      Need configuration tips? Review the proxy auth guide in the docs.
                      {' '}
                      <a
                        class="underline font-medium"
                        href="https://github.com/rcourtman/Pulse/blob/main/docs/PROXY_AUTH.md"
                        target="_blank"
                        rel="noreferrer"
                      >
                        Read proxy auth guide →
                      </a>
                    </p>
                  </div>
                </Card>
              </Show>

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
                      <Card tone="muted" padding="sm" class="mt-3 border border-amber-200 dark:border-amber-700">
                        <p class="text-xs font-semibold text-gray-900 dark:text-gray-100 mb-2">
                          To enable authentication:
                        </p>
                        <ol class="text-xs text-gray-600 dark:text-gray-400 space-y-1">
                          <li>1. Remove DISABLE_AUTH from environment variables</li>
                          <li>2. Delete /etc/pulse/.auth_recovery if it exists</li>
                          <li>3. Restart Pulse service</li>
                          <li>4. Complete the security setup wizard on first access</li>
                        </ol>
                      </Card>
                      <div class="mt-4">
                        <button
                          type="button"
                          onClick={() => setShowQuickSecuritySetup(!showQuickSecuritySetup())}
                          class="px-4 py-2 text-xs font-semibold rounded-lg border border-blue-300 text-blue-700 bg-blue-50 hover:bg-blue-100 transition-colors dark:border-blue-700 dark:text-blue-200 dark:bg-blue-900/30 dark:hover:bg-blue-900/40"
                        >
                          {showQuickSecuritySetup() ? 'Hide quick security setup' : 'Launch quick security setup'}
                        </button>
                      </div>
                      <Show when={showQuickSecuritySetup()}>
                        <div class="mt-4">
                          <QuickSecuritySetup onConfigured={() => {
                            setShowQuickSecuritySetup(false);
                            loadSecurityStatus();
                          }} />
                        </div>
                      </Show>
                    </div>
                  </div>
                </div>
              </Show>
              
              {/* Authentication */}
              <Show when={!securityStatusLoading() && (securityStatus()?.hasAuthentication || securityStatus()?.apiTokenConfigured)}>
                <Card padding="none" class="overflow-hidden border border-gray-200 dark:border-gray-700" border={false}>
                  {/* Header */}
                  <div class="bg-gradient-to-r from-gray-50 to-gray-50 dark:from-gray-900/20 dark:to-gray-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                    <div class="flex items-center gap-3">
                      <div class="p-2 bg-gray-100 dark:bg-gray-900/50 rounded-lg">
                        <svg class="w-5 h-5 text-gray-600 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                        </svg>
                      </div>
                      <SectionHeader
                        title="Authentication"
                        description="Manage your login credentials"
                        size="sm"
                        class="flex-1"
                      />
                    </div>
                  </div>
                  
                  {/* Content */}
                  <div class="p-6">
                    <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                      <div class="flex-1 text-sm text-gray-600 dark:text-gray-400">
                        <p class="font-semibold text-gray-900 dark:text-gray-100">Credential controls</p>
                        <p class="mt-1 leading-relaxed">
                          Update the administrator password for routine maintenance, or rotate both the password and API token when you need a full credential refresh.
                        </p>
                      </div>
                      <div class="flex flex-wrap items-start gap-4">
                        <button
                          type="button"
                          onClick={(e) => {
                            e.preventDefault();
                            e.stopPropagation();
                            setShowPasswordModal(true);
                          }}
                          class="flex items-center gap-3 px-4 py-3 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-all"
                        >
                          <div class="p-2 bg-blue-100 dark:bg-blue-900/30 rounded-lg">
                            <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
                            </svg>
                          </div>
                          <div class="text-left">
                            <div class="text-sm font-medium text-gray-900 dark:text-gray-100">Change password</div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">Keep existing API token</div>
                          </div>
                        </button>
                        <div class="flex flex-col gap-1">
                          <span class="text-xs text-gray-500 dark:text-gray-400">Need to replace both the password and API token?</span>
                          <button
                            type="button"
                            onClick={() => setShowQuickSecurityWizard(!showQuickSecurityWizard())}
                            class={`inline-flex items-center gap-2 text-sm font-medium text-indigo-600 dark:text-indigo-300 hover:text-indigo-700 dark:hover:text-indigo-200 transition-colors ${showQuickSecurityWizard() ? 'underline' : ''}`}
                          >
                            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                            </svg>
                            <span>Generate new credentials</span>
                          </button>
                        </div>
                      </div>
                    </div>

                    <Show when={showQuickSecurityWizard()}>
                      <div class="mt-6">
                        <QuickSecuritySetup
                          mode="rotate"
                          defaultUsername={securityStatus()?.authUsername || 'admin'}
                          onConfigured={() => {
                            setShowQuickSecurityWizard(false);
                            loadSecurityStatus();
                          }}
                        />
                      </div>
                    </Show>

                    <div class="mt-8 grid gap-3 text-xs text-gray-600 dark:text-gray-400 md:grid-cols-2">
                      <div class="flex items-start gap-2">
                        <svg class="w-4 h-4 mt-0.5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5.121 17.804A13.937 13.937 0 0112 15c2.5 0 4.847.655 6.879 1.804M15 10a3 3 0 11-6 0 3 3 0 016 0z" />
                        </svg>
                        <div>
                          <div class="font-medium text-gray-800 dark:text-gray-200">Admin user</div>
                          <div>{securityStatus()?.authUsername || 'Not configured'}</div>
                        </div>
                      </div>
                      <div class="flex items-start gap-2">
                        <svg class="w-4 h-4 mt-0.5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3" />
                        </svg>
                        <div>
                          <div class="font-medium text-gray-800 dark:text-gray-200">Last updated</div>
                          <div>{formatTimestamp(securityStatus()?.authLastModified)}</div>
                        </div>
                      </div>
                      <div class="flex items-start gap-2">
                        <svg class="w-4 h-4 mt-0.5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6l4 2" />
                        </svg>
                        <div>
                          <div class="font-medium text-gray-800 dark:text-gray-200">Current coverage</div>
                          <div>
                            {securityStatus()?.hasAuthentication ? 'Password login required.' : 'Password login disabled.'}
                            {' '}
                            {securityStatus()?.oidcEnabled ? 'OIDC available.' : 'OIDC off.'}
                          </div>
                        </div>
                      </div>
                      <div class="flex items-start gap-2">
                        <svg class="w-4 h-4 mt-0.5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-3-3v6m9 3a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <div>
                          <div class="font-medium text-gray-800 dark:text-gray-200">Disable password auth</div>
                          <div>
                            Confirm OIDC or proxy auth works, then set <code class="px-1 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">DISABLE_AUTH=true</code> in your deployment.
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </Card>
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
              
              <div class="rounded-lg border border-blue-200 dark:border-blue-900/60 bg-blue-50/60 dark:bg-blue-900/10 p-4 mb-6">
                <h4 class="text-sm font-semibold text-blue-800 dark:text-blue-200 mb-1">Single Sign-On</h4>
                <p class="text-xs text-blue-700 dark:text-blue-300">
                  Enable OIDC to add an SSO button alongside the existing password login. Disable password auth later by setting <code class="px-1 py-0.5 bg-blue-100/70 dark:bg-blue-900/40 rounded">DISABLE_AUTH=true</code> once SSO is verified.
                </p>
              </div>

              <OIDCPanel onConfigUpdated={loadSecurityStatus} />

              {/* Security setup now handled by first-run wizard */}

              {/* API Token - Show always to allow API access even when auth is disabled */}
              <Show when={!securityStatusLoading()}>
                <Card padding="none" class="overflow-hidden border border-gray-200 dark:border-gray-700" border={false}>
                  {/* Header */}
                  <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                    <div class="flex items-center gap-3">
                      <div class="p-2 bg-blue-100 dark:bg-blue-900/50 rounded-lg">
                        <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
                        </svg>
                      </div>
                      <SectionHeader
                        title="API token"
                        description="For automation and integrations"
                        size="sm"
                        class="flex-1"
                      />
                    </div>
                  </div>
                  
                  {/* Content */}
                  <div class="p-6">
                    {/* Show explanation when auth is disabled */}
                    <Show when={!securityStatus()?.hasAuthentication}>
                      <Card tone="info" padding="sm" class="mb-4 border border-blue-200 dark:border-blue-800">
                        <p class="text-xs text-blue-800 dark:text-blue-200">
                          <strong>API Access Control:</strong> Even though authentication is disabled, you can still use API tokens to protect API access for automation and integrations.
                        </p>
                      </Card>
                    </Show>
                    <div class="mb-4 space-y-2 text-xs text-gray-600 dark:text-gray-400">
                      <p>
                        Exports and imports {securityStatus()?.exportProtected && !securityStatus()?.unprotectedExportAllowed ? 'require an API token and a passphrase' : 'follow the current server policy'}. Generating a token lets you:
                      </p>
                      <ul class="list-disc pl-5 space-y-1">
                        <li>Authenticate scripts with the <code class="px-1 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">X-API-Token</code> header.</li>
                        <li>Unlock encrypted export/import flows in Settings → Security → Backup &amp; restore.</li>
                        <li>Keep UI logins separate from automation secrets.</li>
                      </ul>
                      <Show when={securityStatus()?.unprotectedExportAllowed}>
                        <p class="text-amber-700 dark:text-amber-300">
                          Unprotected exports are currently allowed. Set <code class="px-1 py-0.5 bg-amber-100 dark:bg-amber-900/50 rounded">ALLOW_UNPROTECTED_EXPORT=false</code> or configure an API token to harden backups.
                        </p>
                      </Show>
                    </div>
                    <GenerateAPIToken currentTokenHint={securityStatus()?.apiTokenHint} />
                  </div>
                </Card>
              </Show>

              {/* Advanced - Only show if auth is enabled */}
              {/* Advanced Options section removed - was only used for Registration Tokens */}
            </div>
          </Show>
          
          {/* Diagnostics Tab */}
          <Show when={activeTab() === 'diagnostics'}>
            <div class="space-y-6">
              <div>
                <SectionHeader title="System diagnostics" size="md" class="mb-4" />
                
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
                        <Card padding="sm">
                          <h5 class="text-sm font-semibold mb-2 text-gray-700 dark:text-gray-300">System</h5>
                          <div class="text-xs space-y-1 text-gray-600 dark:text-gray-400">
                            <div>Version: {diagnosticsData()?.version || 'Unknown'}</div>
                            <div>Uptime: {Math.floor((diagnosticsData()?.uptime || 0) / 60)} minutes</div>
                            <div>Runtime: {diagnosticsData()?.runtime || 'Unknown'}</div>
                            <div>Memory: {Math.round((diagnosticsData()?.system?.memory?.alloc || 0) / 1024 / 1024)} MB</div>
                          </div>
                        </Card>
                        
                        {/* Nodes Status */}
                        <Show when={diagnosticsData()?.nodes && diagnosticsData()!.nodes.length > 0}>
                          <Card padding="sm">
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
                          </Card>
                        </Show>
                        
                        {/* PBS Status */}
                        <Show when={diagnosticsData()?.pbs && diagnosticsData()!.pbs.length > 0}>
                          <Card padding="sm">
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
                          </Card>
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
                      const sanitizeForGitHub = (data: Record<string, unknown>) => {
                        // Deep clone the data
                        const sanitized = JSON.parse(JSON.stringify(data)) as Record<string, unknown>;
                        
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
                          sanitized.nodes = (sanitized.nodes as Array<Record<string, unknown>>).map((node, index: number) => {
                            const nodeType = typeof node.type === 'string' ? (node.type as string) : 'node';
                            const nodeName = typeof node.name === 'string' ? (node.name as string) : '';
                            const nodeHost = typeof node.host === 'string' ? (node.host as string) : '';
                            const tokenName = typeof node.tokenName === 'string' ? (node.tokenName as string) : undefined;
                            const clusterName = typeof node.clusterName === 'string' ? (node.clusterName as string) : undefined;
                            const clusterEndpoints = Array.isArray(node.clusterEndpoints)
                              ? (node.clusterEndpoints as Array<Record<string, unknown>>).map((ep, epIndex: number) => ({
                                  ...ep,
                                  NodeName: `node-${epIndex + 1}`,
                                  Host: `node-${epIndex + 1}`,
                                  IP: sanitizeIP(typeof ep.IP === 'string' ? ep.IP : '')
                                }))
                              : node.clusterEndpoints;

                            return {
                              ...node,
                              id: `${nodeType}-${index}`,
                              name: sanitizeHostname(nodeName),
                              host: nodeHost ? nodeHost.replace(/https?:\/\/[^:\/]+/, 'https://REDACTED') : nodeHost,
                              tokenName: tokenName ? 'token-REDACTED' : tokenName,
                              clusterName: clusterName ? 'cluster-REDACTED' : clusterName,
                              clusterEndpoints
                            };
                          });
                        }
                        
                        // Sanitize storage
                        if (sanitized.storage) {
                          sanitized.storage = (sanitized.storage as Array<Record<string, unknown>>).map((s, index: number) => {
                            const storageNode = typeof s.node === 'string' ? s.node : '';
                            return {
                              ...s,
                              id: `storage-${index}`,
                              node: sanitizeHostname(storageNode),
                              name: `storage-${index}`
                            };
                          });
                        }
                        
                        // Sanitize backups
                        const backups = sanitized.backups as Record<string, unknown> | undefined;
                        if (backups) {
                          // Sanitize PVE backup tasks
                          if (Array.isArray(backups.pveBackupTasks)) {
                            backups.pveBackupTasks = (backups.pveBackupTasks as Array<Record<string, unknown>>).map((b, index: number) => {
                              const backupNode = typeof b.node === 'string' ? b.node : '';
                              const backupVmid = typeof b.vmid === 'number' ? b.vmid : undefined;
                              return {
                                ...b,
                                node: sanitizeHostname(backupNode),
                                storage: `storage-${index}`,
                                vmid: backupVmid !== undefined ? `vm-${backupVmid}` : backupVmid
                              };
                            });
                          }
                          
                          // Sanitize PVE storage backups
                          if (Array.isArray(backups.pveStorageBackups)) {
                            backups.pveStorageBackups = (backups.pveStorageBackups as Array<Record<string, unknown>>).map((b, index: number) => {
                              const backupNode = typeof b.node === 'string' ? b.node : '';
                              const backupVmid = typeof b.vmid === 'number' ? b.vmid : undefined;
                              const volid = typeof b.volid === 'string' ? b.volid : undefined;
                              return {
                                ...b,
                                node: sanitizeHostname(backupNode),
                                storage: `storage-${index}`,
                                vmid: backupVmid !== undefined ? `vm-${backupVmid}` : backupVmid,
                                volid: volid ? 'vol-REDACTED' : volid
                              };
                            });
                          }
                          
                          // Sanitize PBS backups
                          if (Array.isArray(backups.pbsBackups)) {
                            backups.pbsBackups = (backups.pbsBackups as Array<Record<string, unknown>>).map((b, index: number) => {
                              const backupId = typeof b.backupId === 'string' ? b.backupId : undefined;
                              const vmName = typeof b.vmName === 'string' ? b.vmName : undefined;
                              return {
                                ...b,
                                datastore: `datastore-${index}`,
                                backupId: backupId ? `backup-${index}` : backupId,
                                vmName: vmName ? 'vm-REDACTED' : vmName
                              };
                            });
                          }
                        }

                        // Sanitize active alerts
                        const activeAlerts = sanitized.activeAlerts as Array<Record<string, unknown>> | undefined;
                        if (activeAlerts) {
                          sanitized.activeAlerts = activeAlerts.map((alert) => {
                            const alertNode = typeof alert.node === 'string' ? alert.node : '';
                            const details = typeof alert.details === 'string' ? alert.details : undefined;
                            return {
                              ...alert,
                              node: sanitizeHostname(alertNode),
                              details: details ? details.replace(/\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/g, 'xxx.xxx.xxx.xxx') : details
                            };
                          });
                        }

                        // Sanitize websocket URL
                        const websocketInfo = sanitized.websocket as Record<string, unknown> | undefined;
                        if (websocketInfo && typeof websocketInfo.url === 'string') {
                          websocketInfo.url = websocketInfo.url.replace(/\/\/[^\/]+/, '//REDACTED');
                        }
                        
                        // Add sanitization notice
                        sanitized._notice = 'This diagnostic data has been sanitized for sharing on GitHub. IP addresses, hostnames, and tokens have been redacted.';
                        
                        return sanitized;
                      };
                      
                      const exportDiagnostics = (sanitize: boolean) => {
                        let diagnostics: Record<string, unknown> = {
                          timestamp: new Date().toISOString(),
                          version: '2.1.0',
                          pulseVersion: state.stats?.version || 'unknown',
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
                          // Include backend diagnostics if available
                          backendDiagnostics: diagnosticsData() || null,
                          nodes: nodes()?.map(n => ({
                            ...n,
                            status: state.nodes?.find(sn => sn.id === n.id)?.status || 'unknown',
                            online: state.nodes?.find(sn => sn.id === n.id)?.status === 'online'
                          })) || [],
                          state: {
                            nodesCount: state.nodes?.length || 0,
                            nodesOnline: state.nodes?.filter(n => n.status === 'online').length || 0,
                            nodesOffline: state.nodes?.filter(n => n.status !== 'online').length || 0,
                            vmsCount: state.vms?.length || 0,
                            containersCount: state.containers?.length || 0,
                            storageCount: state.storage?.length || 0,
                            physicalDisksCount: state.physicalDisks?.length || 0,
                            pbsCount: state.pbs?.length || 0,
                            pbsBackupsCount: state.pbsBackups?.length || 0,
                            pveBackups: {
                              backupTasksCount: state.pveBackups?.backupTasks?.length || 0,
                              storageBackupsCount: state.pveBackups?.storageBackups?.length || 0,
                              guestSnapshotsCount: state.pveBackups?.guestSnapshots?.length || 0
                            }
                          },
                          // Node status details
                          nodeStatus: state.nodes?.map(n => ({
                            id: n.id,
                            name: n.name,
                            status: n.status,
                            online: n.status === 'online',
                            cpu: n.cpu,
                            memory: n.memory,
                            uptime: n.uptime,
                            version: n.pveVersion ?? n.kernelVersion
                          })) || [],
                          storage: state.storage?.map(s => ({
                            id: s.id,
                            node: s.node,
                            name: s.name,
                            type: s.type,
                            status: s.status,
                            enabled: s.enabled,
                            content: s.content,
                            shared: s.shared,
                            used: s.used,
                            total: s.total,
                            zfsPool: s.zfsPool ? {
                              state: s.zfsPool.state,
                              readErrors: s.zfsPool.readErrors,
                              writeErrors: s.zfsPool.writeErrors,
                              checksumErrors: s.zfsPool.checksumErrors,
                              deviceCount: s.zfsPool.devices?.length || 0
                            } : undefined,
                            hasBackups: (state.pveBackups?.storageBackups?.filter((b) => b.storage === s.name).length || 0) > 0
                          })) || [],
                          // Physical disks - critical for troubleshooting
                          physicalDisks: state.physicalDisks?.map(d => ({
                            node: d.node,
                            device: d.device || d.devPath,
                            model: d.model,
                            size: d.size,
                            type: d.type,
                            health: d.health,
                            wearout: d.wearout,
                            rpm: d.rpm,
                            smart: d.smart ?? null
                          })) || [],
                          backups: {
                            pveBackupTasks: state.pveBackups?.backupTasks?.slice(0, 10) || [],
                            pveStorageBackups: state.pveBackups?.storageBackups?.slice(0, 10) || [],
                            pbsBackups: state.pbsBackups?.slice(0, 10) || []
                          },
                          connectionHealth: state.connectionHealth || {},
                          performance: {
                            lastPollDuration: state.performance?.lastPollDuration || 0,
                            totalApiCalls: state.performance?.totalApiCalls || 0,
                            failedApiCalls: state.performance?.failedApiCalls || 0,
                            apiCallDuration: state.performance?.apiCallDuration || {}
                          },
                          activeAlerts: state.activeAlerts?.slice(0, 20) || [],
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
                        <div class="space-y-3">
                          <Show when={!diagnosticsData()}>
                            <div class="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 rounded p-2">
                              💡 Run diagnostics first for more comprehensive export data
                            </div>
                          </Show>
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
      </Card>
      
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
                      hasToken: nodeData.tokenValue ? true : n.hasToken,
                      status: 'pending'
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
                status: node.status || 'pending' as const
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
                      status: 'pending'
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
                status: node.status || 'pending' as const
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
          <Card padding="lg" class="max-w-md w-full">
            <SectionHeader title="Export configuration" size="md" class="mb-4" />
            
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
              <div class={formField}>
                <label class={labelClass()}>
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
                      ? (useCustomPassphrase() ? 'Enter a strong passphrase' : 'Enter your Pulse login password')
                      : 'Enter a strong passphrase for encryption'
                  }
                  class={controlClass()}
                />
                <Show when={!securityStatus()?.hasAuthentication || useCustomPassphrase()}>
                  <p class={`${formHelpText} mt-1`}>
                    You'll need this passphrase to restore the backup.
                  </p>
                </Show>
                <Show when={securityStatus()?.hasAuthentication && !useCustomPassphrase()}>
                  <p class={`${formHelpText} mt-1`}>
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
          </Card>
        </div>
      </Show>
      
      {/* API Token Modal */}
      <Show when={showApiTokenModal()}>
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card padding="lg" class="max-w-md w-full">
            <SectionHeader title="API token required" size="md" class="mb-4" />
            
            <div class="space-y-4">
              <p class="text-sm text-gray-600 dark:text-gray-400">
                This Pulse instance requires an API token for export/import operations. Please enter the API token configured on the server.
              </p>
              
              <div class={formField}>
                <label class={labelClass()}>
                  API Token
                </label>
                <input
                  type="password"
                  value={apiTokenInput()}
                  onInput={(e) => setApiTokenInput(e.currentTarget.value)}
                  placeholder="Enter API token"
                  class={controlClass()}
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
          </Card>
        </div>
      </Show>
      
      {/* Import Dialog */}
      <Show when={showImportDialog()}>
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card padding="lg" class="max-w-md w-full">
            <SectionHeader title="Import configuration" size="md" class="mb-4" />
            
            <div class="space-y-4">
              <div class={formField}>
                <label class={labelClass()}>
                  Configuration File
                </label>
                <input
                  type="file"
                  accept=".json"
                  onChange={(e) => {
                    const file = e.currentTarget.files?.[0];
                    if (file) setImportFile(file);
                  }}
                  class={controlClass('cursor-pointer')}
                />
              </div>
              
              <div class={formField}>
                <label class={labelClass()}>
                  Backup Password
                </label>
                <input
                  type="password"
                  value={importPassphrase()}
                  onInput={(e) => setImportPassphrase(e.currentTarget.value)}
                  placeholder="Enter the password used when creating this backup"
                  class={controlClass()}
                />
                <p class={`${formHelpText} mt-1`}>
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
          </Card>
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
