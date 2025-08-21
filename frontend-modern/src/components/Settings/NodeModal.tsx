import { Component, Show, createSignal, createEffect } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { NodeConfig } from '@/types/nodes';
import type { SecurityStatus } from '@/types/config';
import { copyToClipboard } from '@/utils/clipboard';
import { showSuccess, showError } from '@/utils/toast';
import { NodesAPI } from '@/api/nodes';

interface NodeModalProps {
  isOpen: boolean;
  resetKey?: number;
  onClose: () => void;
  nodeType: 'pve' | 'pbs';
  editingNode?: NodeConfig;
  onSave: (nodeData: Partial<NodeConfig>) => void;
  showBackToDiscovery?: boolean;
  onBackToDiscovery?: () => void;
  securityStatus?: Partial<SecurityStatus>;
}

export const NodeModal: Component<NodeModalProps> = (props) => {
  const [testResult, setTestResult] = createSignal<{ status: string; message: string; isCluster?: boolean } | null>(null);
  const [isTesting, setIsTesting] = createSignal(false);
  
  // Function to get clean form data
  const getCleanFormData = () => ({
    name: '',
    host: '',
    authType: 'token' as 'password' | 'token',
    setupMode: 'auto' as 'auto' | 'manual',
    user: '',
    password: '',
    tokenName: '',
    tokenValue: '',
    fingerprint: '',
    verifySSL: true,
    // PVE specific
    monitorVMs: true,
    monitorContainers: true,
    monitorStorage: true,
    monitorBackups: true,
    enableBackupManagement: true, // New field for backup write permissions
    // PBS specific
    monitorDatastores: true,
    monitorSyncJobs: true,
    monitorVerifyJobs: true,
    monitorPruneJobs: true,
    monitorGarbageJobs: false
  });
  
  const [formData, setFormData] = createSignal(getCleanFormData());
  const [setupCode, setSetupCode] = createSignal<{code: string, expires: number} | null>(null);

  // Track previous state to detect changes
  let previousResetKey: number | undefined = undefined;
  let previousNodeType: string | undefined = undefined;
  
  // Reset form when conditions change
  createEffect(() => {
    const key = props.resetKey;
    const nodeType = props.nodeType;
    const isOpen = props.isOpen;
    const editingNode = props.editingNode;
    
    // Force reset if resetKey changed
    if (key !== undefined && key !== previousResetKey) {
      previousResetKey = key;
      setFormData(() => getCleanFormData());
      setTestResult(null);
      return;
    }
    
    // Force reset if node type changed
    if (nodeType !== previousNodeType && previousNodeType !== undefined) {
      previousNodeType = nodeType;
      setFormData(() => getCleanFormData());
      setTestResult(null);
      return;
    }
    previousNodeType = nodeType;
    
    // Reset when opening for new node
    if (isOpen && !editingNode) {
      setFormData(() => getCleanFormData());
      setTestResult(null);
    }
  });
  
  // Generate setup URL when Quick Setup tab is shown
  // Skip auto-generation to avoid race conditions - generate on-demand when copy is clicked

  // Update form when editing node changes
  createEffect(() => {
    // Only populate form if we have an editing node AND it matches the current node type
    // This prevents PVE data from being used when adding a PBS node
    if (props.editingNode && props.editingNode.type === props.nodeType) {
      const node = props.editingNode;
      // Handle auth fields
      let username = ('user' in node ? node.user : '') || '';
      let tokenName = node.tokenName || '';
      
      // For PBS with token auth, keep the full token format in tokenName field
      // The user field is not shown in the UI for token auth anyway
      if (props.nodeType === 'pbs' && tokenName && tokenName.includes('!') && !node.hasPassword) {
        // Keep full token format for PBS token auth
        // tokenName stays as-is (e.g., "pulse-monitor@pbs!pulse-192-168-0-123")
        // Extract username for internal use only
        const parts = tokenName.split('!');
        username = parts[0];
      }
      
      setFormData({
        name: node.name || '',
        host: node.host || '',
        authType: node.hasPassword ? 'password' : 'token', // Check actual auth type
        setupMode: 'auto',
        user: username,
        password: '', // Don't show existing password
        tokenName: tokenName,
        tokenValue: '', // Don't show existing token
        fingerprint: ('fingerprint' in node ? node.fingerprint : '') || '',
        verifySSL: node.verifySSL ?? true,
        monitorVMs: (node.type === 'pve' && 'monitorVMs' in node ? node.monitorVMs : true) ?? true,
        monitorContainers: (node.type === 'pve' && 'monitorContainers' in node ? node.monitorContainers : true) ?? true,
        monitorStorage: (node.type === 'pve' && 'monitorStorage' in node ? node.monitorStorage : true) ?? true,
        monitorBackups: (node.type === 'pve' && 'monitorBackups' in node ? node.monitorBackups : true) ?? true,
        enableBackupManagement: true, // Default to true for existing nodes
        monitorDatastores: (node.type === 'pbs' && 'monitorDatastores' in node ? node.monitorDatastores : true) ?? true,
        monitorSyncJobs: (node.type === 'pbs' && 'monitorSyncJobs' in node ? node.monitorSyncJobs : true) ?? true,
        monitorVerifyJobs: (node.type === 'pbs' && 'monitorVerifyJobs' in node ? node.monitorVerifyJobs : true) ?? true,
        monitorPruneJobs: (node.type === 'pbs' && 'monitorPruneJobs' in node ? node.monitorPruneJobs : true) ?? true,
        monitorGarbageJobs: (node.type === 'pbs' && 'monitorGarbageJobs' in node ? node.monitorGarbageJobs : false) ?? false
      });
    }
  });

  const handleSubmit = (e: Event) => {
    e.preventDefault();
    const data = formData();
    
    // Prepare data based on auth type
    const nodeData: Partial<NodeConfig> = {
      type: props.nodeType,
      name: data.name || '', // Will be auto-generated by backend if empty
      host: data.host,
      fingerprint: data.fingerprint,
      verifySSL: data.verifySSL
    };

    if (data.authType === 'password') {
      nodeData.user = data.user;
      if (data.password) {
        nodeData.password = data.password;
      }
    } else {
      // For token auth, tokenName should already contain the full token ID
      nodeData.tokenName = data.tokenName;
      if (data.tokenValue) {
        nodeData.tokenValue = data.tokenValue;
      }
    }

    // Add monitor settings based on type
    if (props.nodeType === 'pve') {
      Object.assign(nodeData, {
        monitorVMs: data.monitorVMs,
        monitorContainers: data.monitorContainers,
        monitorStorage: data.monitorStorage,
        monitorBackups: data.monitorBackups
      });
    } else {
      Object.assign(nodeData, {
        monitorDatastores: data.monitorDatastores,
        monitorSyncJobs: data.monitorSyncJobs,
        monitorVerifyJobs: data.monitorVerifyJobs,
        monitorPruneJobs: data.monitorPruneJobs,
        monitorGarbageJobs: data.monitorGarbageJobs
      });
    }

    props.onSave(nodeData);
  };

  const updateField = (field: string, value: string | boolean) => {
    setFormData(prev => ({ ...prev, [field]: value }));
  };

  const handleTestConnection = async () => {
    const data = formData();
    
    // If editing an existing node and no new credentials provided, use stored credentials
    if (props.editingNode) {
      const hasNewPassword = data.authType === 'password' && data.password;
      const hasNewToken = data.authType === 'token' && data.tokenValue;
      
      if (!hasNewPassword && !hasNewToken) {
        // Use the existing node test endpoint which uses stored credentials
        setIsTesting(true);
        setTestResult(null);
        
        try {
          const result = await NodesAPI.testExistingNode(props.editingNode.id);
          setTestResult({
            status: 'success',
            message: result.message || 'Connection successful'
          });
        } catch (error) {
          setTestResult({
            status: 'error',
            message: error instanceof Error ? error.message : 'Connection failed'
          });
        } finally {
          setIsTesting(false);
        }
        return;
      }
    }
    
    // Validate required fields for new nodes or when new credentials are provided
    if (!data.host) {
      setTestResult({ status: 'error', message: 'Host is required' });
      return;
    }
    
    if (data.authType === 'password' && (!data.user || !data.password)) {
      setTestResult({ status: 'error', message: 'Username and password are required' });
      return;
    }
    
    if (data.authType === 'token' && (!data.tokenName || !data.tokenValue)) {
      setTestResult({ status: 'error', message: 'Token ID and token value are required' });
      return;
    }
    
    // Prepare test data
    const testData: Partial<NodeConfig> = {
      type: props.nodeType,
      name: data.name || '', // Will be auto-generated by backend if empty
      host: data.host,
      fingerprint: data.fingerprint,
      verifySSL: data.verifySSL
    };
    
    if (data.authType === 'password') {
      testData.user = data.user;
      testData.password = data.password;
    } else {
      // For token auth, tokenName contains the full token ID
      testData.tokenName = data.tokenName;
      testData.tokenValue = data.tokenValue;
    }
    
    setIsTesting(true);
    setTestResult(null);
    
    try {
      const result = await NodesAPI.testConnection(testData as NodeConfig);
      setTestResult({
        status: 'success',
        message: result.message || 'Connection successful',
        isCluster: result.isCluster
      });
    } catch (error) {
      setTestResult({
        status: 'error',
        message: error instanceof Error ? error.message : 'Connection failed'
      });
    } finally {
      setIsTesting(false);
    }
  };

  return (
    <Portal>
      <Show when={props.isOpen}>
        <div class="fixed inset-0 z-50 overflow-y-auto">
          <div class="flex min-h-screen items-center justify-center p-4">
            {/* Backdrop */}
            <div 
              class="fixed inset-0 bg-black/50 transition-opacity"
              onClick={props.onClose}
            />
            
            {/* Modal */}
            <div class="relative w-full max-w-2xl bg-white dark:bg-gray-800 rounded-lg shadow-xl">
              <form onSubmit={handleSubmit}>
                {/* Header */}
                <div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
                  <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
                    {props.editingNode ? 'Edit' : 'Add'} {props.nodeType === 'pve' ? 'Proxmox VE' : 'Proxmox Backup Server'} Node
                  </h3>
                  <button
                    type="button"
                    onClick={props.onClose}
                    class="text-gray-400 hover:text-gray-500 dark:hover:text-gray-300"
                  >
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <line x1="18" y1="6" x2="6" y2="18"></line>
                      <line x1="6" y1="6" x2="18" y2="18"></line>
                    </svg>
                  </button>
                </div>
                
                {/* Body */}
                <div class="p-6 space-y-6">
                  {/* Basic Information */}
                  <div>
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4">Basic Information</h4>
                    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                          Node Name <span class="text-gray-400">(optional)</span>
                        </label>
                        <input
                          type="text"
                          value={formData().name}
                          onInput={(e) => updateField('name', e.currentTarget.value)}
                          placeholder="Will auto-detect from hostname"
                          class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                        />
                      </div>
                      
                      <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                          Host URL <span class="text-red-500">*</span>
                        </label>
                        <input
                          type="text"
                          value={formData().host}
                          onInput={(e) => updateField('host', e.currentTarget.value)}
                          placeholder={props.nodeType === 'pve' ? "https://proxmox.example.com:8006" : "https://backup.example.com:8007"}
                          required
                          class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                        />
                        <Show when={props.nodeType === 'pbs'}>
                          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">PBS requires HTTPS (not HTTP). Default port is 8007</p>
                        </Show>
                      </div>
                    </div>
                  </div>
                  
                  {/* Authentication */}
                  <div>
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4">Authentication</h4>
                    
                    {/* Auth Type Selector */}
                    <div class="mb-4">
                      <div class="flex gap-4">
                        <label class="flex items-center">
                          <input
                            type="radio"
                            name="authType"
                            value="password"
                            checked={formData().authType === 'password'}
                            onChange={() => updateField('authType', 'password')}
                            class="mr-2"
                          />
                          <span class="text-sm text-gray-700 dark:text-gray-300">Username & Password</span>
                        </label>
                        <label class="flex items-center">
                          <input
                            type="radio"
                            name="authType"
                            value="token"
                            checked={formData().authType === 'token'}
                            onChange={() => updateField('authType', 'token')}
                            class="mr-2"
                          />
                          <span class="text-sm text-gray-700 dark:text-gray-300">API Token <span class="text-green-600 dark:text-green-400 text-xs ml-1">(Recommended)</span></span>
                        </label>
                      </div>
                    </div>
                    
                    {/* Password Auth Fields */}
                    <Show when={formData().authType === 'password'}>
                      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div>
                          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Username <span class="text-red-500">*</span>
                          </label>
                          <input
                            type="text"
                            value={formData().user}
                            onInput={(e) => updateField('user', e.currentTarget.value)}
                            placeholder={props.nodeType === 'pve' ? "root@pam" : "admin@pbs"}
                            required={formData().authType === 'password'}
                            class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                          />
                          <Show when={props.nodeType === 'pbs'}>
                            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Must include realm (e.g., admin@pbs)</p>
                          </Show>
                        </div>
                        
                        <div>
                          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Password {!props.editingNode && <span class="text-red-500">*</span>}
                          </label>
                          <input
                            type="password"
                            value={formData().password}
                            onInput={(e) => updateField('password', e.currentTarget.value)}
                            placeholder={props.editingNode ? 'Leave blank to keep existing' : 'Password'}
                            required={formData().authType === 'password' && !props.editingNode}
                            class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                          />
                        </div>
                      </div>
                    </Show>
                    
                    {/* Token Auth Fields */}
                    <Show when={formData().authType === 'token'}>
                      <div class="space-y-4">
                        {/* Token Creation Guide */}
                        <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
                          <h5 class="text-sm font-medium text-blue-900 dark:text-blue-100 mb-3 flex items-center gap-2">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                              <circle cx="12" cy="12" r="10"></circle>
                              <path d="M12 6v6l4 2"></path>
                            </svg>
                            Quick Token Setup
                          </h5>
                          
                          <Show when={props.nodeType === 'pve'}>
                            <div class="space-y-3 text-xs">
                              {/* Tab buttons */}
                              <div class="flex gap-2 border-b border-gray-200 dark:border-gray-700">
                                <button
                                  type="button"
                                  onClick={() => updateField('setupMode', 'auto')}
                                  class={`px-3 py-1.5 text-sm font-medium border-b-2 transition-colors ${
                                    formData().setupMode === 'auto' 
                                      ? 'border-blue-500 text-blue-600 dark:text-blue-400' 
                                      : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400'
                                  }`}
                                >
                                  Quick Setup
                                </button>
                                <button
                                  type="button"
                                  onClick={() => updateField('setupMode', 'manual')}
                                  class={`px-3 py-1.5 text-sm font-medium border-b-2 transition-colors ${
                                    formData().setupMode === 'manual' 
                                      ? 'border-blue-500 text-blue-600 dark:text-blue-400' 
                                      : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400'
                                  }`}
                                >
                                  Manual Setup
                                </button>
                              </div>

                              {/* Quick Setup Tab */}
                              <Show when={formData().setupMode === 'auto' || !formData().setupMode}>
                                {/* Backup Management Checkbox */}
                                <div class="mb-3">
                                  <label class="flex items-center gap-2 text-sm">
                                    <input
                                      type="checkbox"
                                      checked={formData().enableBackupManagement}
                                      onChange={(e) => setFormData({ ...formData(), enableBackupManagement: e.currentTarget.checked })}
                                      class="rounded border-gray-300 dark:border-gray-600"
                                    />
                                    <span class="text-gray-700 dark:text-gray-300">
                                      Enable storage permissions for backup visibility
                                    </span>
                                  </label>
                                  <p class="text-xs text-gray-500 dark:text-gray-400 ml-6 mt-1">
                                    {formData().enableBackupManagement 
                                      ? 'Required to read PVE backup files and display them in the Backups tab (Proxmox API limitation)'
                                      : 'Backups tab will not show PVE backups without these permissions'}
                                  </p>
                                </div>

                                <p class="text-blue-800 dark:text-blue-200">Just copy and run this one command on your Proxmox VE server:</p>
                                
                                {/* One-line command */}
                                <div class="space-y-3">
                                  <div class="relative bg-gray-900 rounded-md p-3 font-mono text-xs overflow-x-auto">
                                    <button
                                      type="button"
                                      onClick={async () => {
                                        try {
                                          // Check if host is populated
                                          if (!formData().host || formData().host.trim() === '') {
                                            showError('Please enter the Host URL first');
                                            return;
                                          }
                                          
                                          // Always regenerate URL when host changes
                                          const { apiFetch } = await import('@/utils/apiClient');
                                          const response = await apiFetch('/api/setup-script-url', {
                                            method: 'POST',
                                            headers: { 'Content-Type': 'application/json' },
                                            body: JSON.stringify({
                                              type: 'pve',
                                              host: formData().host,
                                              backupPerms: formData().enableBackupManagement
                                            })
                                          });
                                          
                                          if (response.ok) {
                                            const data = await response.json();
                                            const cmd = `curl -sSL "${data.url}" | bash`;
                                            
                                            // Store setup code for display
                                            if (data.setupCode) {
                                              setSetupCode({code: data.setupCode, expires: data.expires});
                                            }
                                            
                                            if (await copyToClipboard(cmd)) {
                                              showSuccess('Command copied to clipboard!');
                                            }
                                          } else {
                                            showError('Failed to generate setup URL');
                                          }
                                        } catch (error) {
                                          console.error('Failed to copy command:', error);
                                          showError('Failed to copy command');
                                        }
                                      }}
                                      class="absolute top-2 right-2 p-1.5 text-gray-400 hover:text-gray-200 bg-gray-700 rounded-md transition-colors"
                                      title="Copy command"
                                    >
                                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                        <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                        <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                      </svg>
                                    </button>
                                    <code class={formData().host ? "text-green-400" : "text-gray-500"}>
                                      {formData().host 
                                        ? 'Click the copy button to generate the secure command'
                                        : '⚠️ Please enter the Host URL above first'}
                                    </code>
                                  </div>
                                  
                                  {/* Setup Code Display */}
                                  <Show when={setupCode()}>
                                    <div class="bg-gradient-to-r from-blue-600 to-purple-600 rounded-lg p-4 text-white">
                                      <div class="flex items-center justify-between mb-3">
                                        <h4 class="text-sm font-semibold flex items-center">
                                          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                                          </svg>
                                          Setup Code (One-Time Use)
                                        </h4>
                                        <button
                                          onClick={() => setSetupCode(null)}
                                          class="text-white/80 hover:text-white"
                                        >
                                          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                          </svg>
                                        </button>
                                      </div>
                                      <div class="bg-white/10 backdrop-blur rounded-md p-3 font-mono text-2xl text-center tracking-wider">
                                        {setupCode()?.code}
                                      </div>
                                      <div class="mt-3 text-xs text-white/90 space-y-1">
                                        <p>✅ Command copied to clipboard</p>
                                        <p>📋 Enter this code when the setup script prompts you</p>
                                        <p>⏱️ Expires in 5 minutes</p>
                                      </div>
                                    </div>
                                  </Show>
                                  
                                  <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-3">
                                    <div class="flex items-start space-x-2">
                                      <svg class="h-5 w-5 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                                      </svg>
                                      <div class="text-xs text-amber-700 dark:text-amber-300">
                                        <p class="font-semibold mb-1">If the command doesn't work:</p>
                                        <p>Your Proxmox server may not be able to reach Pulse. Use the alternative method below.</p>
                                      </div>
                                    </div>
                                  </div>

                                  {/* Alternative: Download script */}
                                  <details class="bg-gray-50 dark:bg-gray-800/50 rounded-lg p-3">
                                    <summary class="cursor-pointer text-sm font-medium text-gray-700 dark:text-gray-300 hover:text-gray-900 dark:hover:text-gray-100">
                                      Alternative: Download script manually
                                    </summary>
                                    <div class="mt-3 space-y-3">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          try {
                                            const hostValue = formData().host || '';
                                            const encodedHost = encodeURIComponent(hostValue);
                                            const pulseUrl = encodeURIComponent(window.location.origin);
                                            const backupPerms = formData().enableBackupManagement ? '&backup_perms=true' : '';
                                            const scriptUrl = `/api/setup-script?type=pve&host=${encodedHost}&pulse_url=${pulseUrl}${backupPerms}`;
                                            
                                            // Fetch the script using the current session
                                            const response = await fetch(scriptUrl);
                                            if (!response.ok) {
                                              throw new Error('Failed to fetch setup script');
                                            }
                                            const scriptContent = await response.text();
                                            
                                            // Create a blob and download
                                            const blob = new Blob([scriptContent], { type: 'text/plain' });
                                            const url = URL.createObjectURL(blob);
                                            const a = document.createElement('a');
                                            a.href = url;
                                            a.download = 'pulse-setup.sh';
                                            document.body.appendChild(a);
                                            a.click();
                                            document.body.removeChild(a);
                                            URL.revokeObjectURL(url);
                                            
                                            showSuccess('Script downloaded! Upload it to your server and run: bash pulse-setup.sh');
                                          } catch (error) {
                                            console.error('Failed to download script:', error);
                                            showSuccess('Failed to download script. Please check your connection.');
                                          }
                                        }}
                                        class="w-full px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md transition-colors text-sm font-medium"
                                      >
                                        Download pulse-setup.sh
                                      </button>
                                      <div class="text-xs text-gray-600 dark:text-gray-400">
                                        1. Click to download the script<br/>
                                        2. Upload to your server via SCP/SFTP<br/>
                                        3. Run: <code class="bg-gray-100 dark:bg-gray-800 px-1 rounded">bash pulse-setup.sh</code>
                                      </div>
                                    </div>
                                  </details>
                                </div>
                              
                              <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-3">
                                <p class="text-sm font-semibold text-blue-800 dark:text-blue-200 mb-2">What this does:</p>
                                <ul class="text-xs text-blue-700 dark:text-blue-300 space-y-1">
                                  <li class="flex items-start">
                                    <span class="text-green-500 mr-2 mt-0.5">✓</span>
                                    <span>Creates monitoring user <code class="bg-blue-100 dark:bg-blue-800/50 px-1 rounded">pulse-monitor@pam</code></span>
                                  </li>
                                  <li class="flex items-start">
                                    <span class="text-green-500 mr-2 mt-0.5">✓</span>
                                    <span>Generates secure API token</span>
                                  </li>
                                  <li class="flex items-start">
                                    <span class="text-green-500 mr-2 mt-0.5">✓</span>
                                    <span>Sets up monitoring permissions (PVEAuditor{formData().enableBackupManagement ? ' + backup access' : ''})</span>
                                  </li>
                                  <li class="flex items-start">
                                    <span class="text-green-500 mr-2 mt-0.5">✓</span>
                                    <span>Automatically registers node with Pulse</span>
                                  </li>
                                </ul>
                                <p class="text-xs text-green-600 dark:text-green-400 mt-2 font-semibold">
                                  ✨ Fully automatic - no manual token copying needed!
                                </p>
                              </div>
                            </Show>
                            
                            {/* Manual Setup Tab */}
                            <Show when={formData().setupMode === 'manual'}>
                              <p class="text-blue-800 dark:text-blue-200 mb-2">Run these commands one by one on your Proxmox VE server:</p>
                              
                              <div class="space-y-3">
                                {/* Step 1: Create user */}
                                <div>
                                  <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">1. Create monitoring user:</p>
                                  <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs">
                                    <button
                                      type="button"
                                      onClick={async () => {
                                        const cmd = 'pveum user add pulse-monitor@pam --comment "Pulse monitoring service"';
                                        if (await copyToClipboard(cmd)) {
                                          showSuccess('Command copied!');
                                        }
                                      }}
                                      class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                      title="Copy command"
                                    >
                                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                        <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                        <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                      </svg>
                                    </button>
                                    <code class="text-gray-800 dark:text-gray-200">pveum user add pulse-monitor@pam --comment "Pulse monitoring service"</code>
                                  </div>
                                </div>
                                
                                {/* Step 2: Generate token */}
                                <div>
                                  <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">2. Generate API token (save the output!):</p>
                                  <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs">
                                    <button
                                      type="button"
                                      onClick={async () => {
                                        const cmd = 'pveum user token add pulse-monitor@pam pulse-token --privsep 0';
                                        if (await copyToClipboard(cmd)) {
                                          showSuccess('Command copied!');
                                        }
                                      }}
                                      class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                      title="Copy command"
                                    >
                                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                        <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                        <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                      </svg>
                                    </button>
                                    <code class="text-gray-800 dark:text-gray-200">pveum user token add pulse-monitor@pam pulse-token --privsep 0</code>
                                  </div>
                                  <p class="text-amber-600 dark:text-amber-400 text-xs mt-1">
                                    ⚠️ Copy the token value immediately - it won't be shown again!
                                  </p>
                                </div>
                                
                                {/* Step 3: Set permissions */}
                                <div>
                                  <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">3. Set up monitoring permissions:</p>
                                  <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs mb-1">
                                    <button
                                      type="button"
                                      onClick={async () => {
                                        const cmd = 'pveum aclmod / -user pulse-monitor@pam -role PVEAuditor';
                                        if (await copyToClipboard(cmd)) {
                                          showSuccess('Command copied!');
                                        }
                                      }}
                                      class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                      title="Copy command"
                                    >
                                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                        <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                        <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                      </svg>
                                    </button>
                                    <code class="text-gray-800 dark:text-gray-200">pveum aclmod / -user pulse-monitor@pam -role PVEAuditor</code>
                                  </div>
                                  <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs">
                                    <button
                                      type="button"
                                      onClick={async () => {
                                        const cmd = 'pveum aclmod /storage -user pulse-monitor@pam -role PVEDatastoreAdmin';
                                        if (await copyToClipboard(cmd)) {
                                          showSuccess('Command copied!');
                                        }
                                      }}
                                      class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                      title="Copy command"
                                    >
                                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                        <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                        <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                      </svg>
                                    </button>
                                    <code class="text-gray-800 dark:text-gray-200">pveum aclmod /storage -user pulse-monitor@pam -role PVEDatastoreAdmin</code>
                                  </div>
                                  <p class="text-gray-600 dark:text-gray-400 text-xs mt-1">
                                    ℹ️ PVEAuditor gives read-only access. PVEDatastoreAdmin on /storage adds backup management capabilities.
                                  </p>
                                </div>
                                
                                {/* Step 4: Use in Pulse */}
                                <div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-md p-2">
                                  <p class="text-sm font-medium text-green-900 dark:text-green-100 mb-1">4. Add to Pulse with:</p>
                                  <ul class="text-xs text-green-800 dark:text-green-200 ml-4 list-disc">
                                    <li><strong>Token ID:</strong> pulse-monitor@pam!pulse-token</li>
                                    <li><strong>Token Value:</strong> [The value from step 2]</li>
                                    <li><strong>Host URL:</strong> {formData().host || 'https://your-server:8006'}</li>
                                  </ul>
                                </div>
                              </div>
                            </Show>
                            </div>
                          </Show>
                          
                          <Show when={props.nodeType === 'pbs'}>
                            <div class="space-y-3 text-xs">
                              {/* Tab buttons for PBS */}
                              <div class="flex gap-2 border-b border-gray-200 dark:border-gray-700">
                                <button
                                  type="button"
                                  onClick={() => updateField('setupMode', 'auto')}
                                  class={`px-3 py-1.5 text-sm font-medium border-b-2 transition-colors ${
                                    formData().setupMode === 'auto' || !formData().setupMode
                                      ? 'border-blue-500 text-blue-600 dark:text-blue-400' 
                                      : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400'
                                  }`}
                                >
                                  Quick Setup
                                </button>
                                <button
                                  type="button"
                                  onClick={() => updateField('setupMode', 'manual')}
                                  class={`px-3 py-1.5 text-sm font-medium border-b-2 transition-colors ${
                                    formData().setupMode === 'manual' 
                                      ? 'border-blue-500 text-blue-600 dark:text-blue-400' 
                                      : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400'
                                  }`}
                                >
                                  Manual Setup
                                </button>
                              </div>

                              {/* Quick Setup Tab for PBS */}
                              <Show when={formData().setupMode === 'auto' || !formData().setupMode}>
                                <p class="text-blue-800 dark:text-blue-200">Just copy and run this one command on your Proxmox Backup Server:</p>
                                
                                {/* One-line command */}
                                <div class="space-y-3">
                                  <div class="relative bg-gray-900 rounded-md p-3 font-mono text-xs overflow-x-auto">
                                    <Show when={formData().host && formData().host.trim() !== ''}>
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          try {
                                            // Check if host is populated
                                            if (!formData().host || formData().host.trim() === '') {
                                              showError('Please enter the Host URL first');
                                              return;
                                            }
                                            
                                            // Always regenerate URL when host changes
                                            const { apiFetch } = await import('@/utils/apiClient');
                                            const response = await apiFetch('/api/setup-script-url', {
                                              method: 'POST',
                                              headers: { 'Content-Type': 'application/json' },
                                              body: JSON.stringify({
                                                type: 'pbs',
                                                host: formData().host,
                                                backupPerms: false
                                              })
                                            });
                                            
                                            if (response.ok) {
                                              const data = await response.json();
                                              const cmd = `curl -sSL "${data.url}" | bash`;
                                              
                                              // Store setup code for display
                                              if (data.setupCode) {
                                                setSetupCode({code: data.setupCode, expires: data.expires});
                                              }
                                              
                                              if (await copyToClipboard(cmd)) {
                                                showSuccess('Command copied to clipboard!');
                                              }
                                            } else {
                                              showError('Failed to generate setup URL');
                                            }
                                          } catch (error) {
                                            console.error('Failed to copy command:', error);
                                            showError('Failed to copy command');
                                          }
                                        }}
                                        class="absolute top-2 right-2 p-1.5 text-gray-400 hover:text-gray-200 bg-gray-700 rounded-md transition-colors"
                                        title="Copy command"
                                      >
                                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                          <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                      </button>
                                    </Show>
                                    <code class={formData().host ? "text-green-400" : "text-gray-500"}>
                                      {formData().host 
                                        ? 'curl -sSL "<click-copy-to-generate-secure-url>" | bash'
                                        : '⚠️ Please enter the Host URL above first'}
                                    </code>
                                  </div>
                                  
                                  {/* Setup Code Display */}
                                  <Show when={setupCode()}>
                                    <div class="bg-gradient-to-r from-blue-600 to-purple-600 rounded-lg p-4 text-white">
                                      <div class="flex items-center justify-between mb-3">
                                        <h4 class="text-sm font-semibold flex items-center">
                                          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                                          </svg>
                                          Setup Code (One-Time Use)
                                        </h4>
                                        <button
                                          onClick={() => setSetupCode(null)}
                                          class="text-white/80 hover:text-white"
                                        >
                                          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                          </svg>
                                        </button>
                                      </div>
                                      <div class="bg-white/10 backdrop-blur rounded-md p-3 font-mono text-2xl text-center tracking-wider">
                                        {setupCode()?.code}
                                      </div>
                                      <div class="mt-3 text-xs text-white/90 space-y-1">
                                        <p>✅ Command copied to clipboard</p>
                                        <p>📋 Enter this code when the setup script prompts you</p>
                                        <p>⏱️ Expires in 5 minutes</p>
                                      </div>
                                    </div>
                                  </Show>
                                  
                                  <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-3">
                                    <div class="flex items-start space-x-2">
                                      <svg class="h-5 w-5 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                                      </svg>
                                      <div class="text-xs text-amber-700 dark:text-amber-300">
                                        <p class="font-semibold mb-1">If the command doesn't work:</p>
                                        <p>Your PBS server may not be able to reach Pulse. Use the alternative method below.</p>
                                      </div>
                                    </div>
                                  </div>

                                  {/* Alternative: Download script */}
                                  <details class="bg-gray-50 dark:bg-gray-800/50 rounded-lg p-3">
                                    <summary class="cursor-pointer text-sm font-medium text-gray-700 dark:text-gray-300 hover:text-gray-900 dark:hover:text-gray-100">
                                      Alternative: Download script manually
                                    </summary>
                                    <div class="mt-3 space-y-3">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          try {
                                            const hostValue = formData().host || '';
                                            const encodedHost = encodeURIComponent(hostValue);
                                            const pulseUrl = encodeURIComponent(window.location.origin);
                                            const scriptUrl = `/api/setup-script?type=pbs&host=${encodedHost}&pulse_url=${pulseUrl}`;
                                            
                                            // Fetch the script using the current session
                                            const response = await fetch(scriptUrl);
                                            if (!response.ok) {
                                              throw new Error('Failed to fetch setup script');
                                            }
                                            const scriptContent = await response.text();
                                          
                                          // Create a blob and download
                                          const blob = new Blob([scriptContent], { type: 'text/plain' });
                                          const url = URL.createObjectURL(blob);
                                          const a = document.createElement('a');
                                          a.href = url;
                                          a.download = 'pulse-pbs-setup.sh';
                                          document.body.appendChild(a);
                                          a.click();
                                          document.body.removeChild(a);
                                          URL.revokeObjectURL(url);
                                          
                                          showSuccess('Script downloaded! Upload it to your PBS and run: bash pulse-pbs-setup.sh');
                                        } catch (error) {
                                          console.error('Failed to download script:', error);
                                          showSuccess('Failed to download script. Please check your connection.');
                                        }
                                      }}
                                      class="w-full px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md transition-colors text-sm font-medium"
                                    >
                                      Download pulse-pbs-setup.sh
                                    </button>
                                    <div class="text-xs text-gray-600 dark:text-gray-400">
                                      1. Click to download the script<br/>
                                      2. Upload to your PBS via SCP/SFTP<br/>
                                      3. Run: <code class="bg-gray-100 dark:bg-gray-800 px-1 rounded">bash pulse-pbs-setup.sh</code>
                                    </div>
                                  </div>
                                </details>
                                </div>
                                
                                <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-3">
                                  <p class="text-sm font-semibold text-blue-800 dark:text-blue-200 mb-2">What this does:</p>
                                  <ul class="text-xs text-blue-700 dark:text-blue-300 space-y-1">
                                    <li class="flex items-start">
                                      <span class="text-green-500 mr-2 mt-0.5">✓</span>
                                      <span>Creates monitoring user <code class="bg-blue-100 dark:bg-blue-800/50 px-1 rounded">pulse-monitor@pbs</code></span>
                                    </li>
                                    <li class="flex items-start">
                                      <span class="text-green-500 mr-2 mt-0.5">✓</span>
                                      <span>Generates secure API token</span>
                                    </li>
                                    <li class="flex items-start">
                                      <span class="text-green-500 mr-2 mt-0.5">✓</span>
                                      <span>Sets up Audit permissions (read-only access to backups + system stats)</span>
                                    </li>
                                    <li class="flex items-start">
                                      <span class="text-green-500 mr-2 mt-0.5">✓</span>
                                      <span>Automatically registers server with Pulse</span>
                                    </li>
                                  </ul>
                                  <p class="text-xs text-green-600 dark:text-green-400 mt-2 font-semibold">
                                    ✨ Fully automatic - no manual token copying needed!
                                  </p>
                                </div>
                              </Show>
                              
                              {/* Manual Setup Tab for PBS */}
                              <Show when={formData().setupMode === 'manual'}>
                                <p class="text-blue-800 dark:text-blue-200 mb-2">Run these commands one by one on your Proxmox Backup Server:</p>
                                
                                <div class="space-y-3">
                                  {/* Step 1: Create user */}
                                  <div>
                                    <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">1. Create monitoring user:</p>
                                    <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd = 'proxmox-backup-manager user create pulse-monitor@pbs';
                                          if (await copyToClipboard(cmd)) {
                                            showSuccess('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                        title="Copy command"
                                      >
                                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                          <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                      </button>
                                      <code class="text-gray-800 dark:text-gray-200">proxmox-backup-manager user create pulse-monitor@pbs</code>
                                    </div>
                                  </div>
                                  
                                  {/* Step 2: Generate token */}
                                  <div>
                                    <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">2. Generate API token (save the output!):</p>
                                    <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd = 'proxmox-backup-manager user generate-token pulse-monitor@pbs pulse-token';
                                          if (await copyToClipboard(cmd)) {
                                            showSuccess('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                        title="Copy command"
                                      >
                                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                          <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                      </button>
                                      <code class="text-gray-800 dark:text-gray-200">proxmox-backup-manager user generate-token pulse-monitor@pbs pulse-token</code>
                                    </div>
                                    <p class="text-amber-600 dark:text-amber-400 text-xs mt-1">
                                      ⚠️ Copy the token value immediately - it won't be shown again!
                                    </p>
                                  </div>
                                  
                                  {/* Step 3: Set permissions */}
                                  <div>
                                    <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">3. Set up read-only permissions (includes system stats):</p>
                                    <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs mb-1">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd = 'proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs';
                                          if (await copyToClipboard(cmd)) {
                                            showSuccess('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                        title="Copy command"
                                      >
                                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                          <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                      </button>
                                      <code class="text-gray-800 dark:text-gray-200">proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs</code>
                                    </div>
                                    <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd = "proxmox-backup-manager acl update / Audit --auth-id 'pulse-monitor@pbs!pulse-token'";
                                          if (await copyToClipboard(cmd)) {
                                            showSuccess('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                        title="Copy command"
                                      >
                                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                          <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                      </button>
                                      <code class="text-gray-800 dark:text-gray-200">proxmox-backup-manager acl update / Audit --auth-id 'pulse-monitor@pbs!pulse-token'</code>
                                    </div>
                                  </div>
                                  
                                  {/* Step 4: Use in Pulse */}
                                  <div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-md p-2">
                                    <p class="text-sm font-medium text-green-900 dark:text-green-100 mb-1">4. Add to Pulse with:</p>
                                    <ul class="text-xs text-green-800 dark:text-green-200 ml-4 list-disc">
                                      <li><strong>Token ID:</strong> pulse-monitor@pbs!pulse-token</li>
                                      <li><strong>Token Value:</strong> [The value from step 2]</li>
                                      <li><strong>Host URL:</strong> {formData().host || 'https://your-server:8007'}</li>
                                    </ul>
                                  </div>
                                  
                                  {/* Permission Info Box */}
                                  <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-md p-2 mt-3">
                                    <p class="text-xs font-semibold text-amber-800 dark:text-amber-200 mb-1">About PBS Permissions:</p>
                                    <ul class="text-xs text-amber-700 dark:text-amber-300 space-y-0.5">
                                      <li><strong>Basic (DatastoreAudit):</strong> View backups only</li>
                                      <li><strong>Enhanced (Audit on /):</strong> View backups + CPU/memory/uptime stats</li>
                                      <li class="text-amber-600 dark:text-amber-400">→ We use Enhanced for better monitoring visibility</li>
                                    </ul>
                                  </div>
                                </div>
                              </Show>
                            </div>
                          </Show>
                        </div>
                        
                        {/* Token Input Fields */}
                        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                          <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                              Token ID <span class="text-red-500">*</span>
                            </label>
                            <input
                              type="text"
                              value={formData().tokenName}
                              onInput={(e) => updateField('tokenName', e.currentTarget.value)}
                              placeholder={props.nodeType === 'pve' ? 'pulse-monitor@pam!pulse-token' : 'pulse-monitor@pbs!pulse-token'}
                              required={formData().authType === 'token'}
                              class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 font-mono"
                            />
                            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Full token ID from Proxmox (user@realm!tokenname)</p>
                          </div>
                          
                          <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                              Token Value {!props.editingNode && <span class="text-red-500">*</span>}
                            </label>
                            <input
                              type="password"
                              value={formData().tokenValue}
                              onInput={(e) => updateField('tokenValue', e.currentTarget.value)}
                              placeholder={props.editingNode ? 'Leave blank to keep existing' : 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'}
                              required={formData().authType === 'token' && !props.editingNode}
                              class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 font-mono"
                            />
                            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">The secret value shown when creating the token</p>
                          </div>
                        </div>
                      </div>
                    </Show>
                  </div>
                  
                  {/* SSL Settings */}
                  <div>
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4">SSL Settings</h4>
                    <div class="space-y-3">
                      <div class="flex items-center">
                        <input
                          type="checkbox"
                          id="verifySSL"
                          checked={formData().verifySSL}
                          onChange={(e) => updateField('verifySSL', e.currentTarget.checked)}
                          class="mr-2"
                        />
                        <label for="verifySSL" class="text-sm text-gray-700 dark:text-gray-300">
                          Verify SSL Certificate
                        </label>
                      </div>
                      
                      <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                          SSL Fingerprint (Optional)
                        </label>
                        <input
                          type="text"
                          value={formData().fingerprint}
                          onInput={(e) => updateField('fingerprint', e.currentTarget.value)}
                          placeholder="AA:BB:CC:DD:EE:FF:..."
                          class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 font-mono"
                        />
                      </div>
                    </div>
                  </div>
                  
                  {/* Monitoring Options */}
                  <div>
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4">Monitoring Options</h4>
                    <div class="space-y-2">
                      {props.nodeType === 'pve' ? (
                        <>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorVMs}
                              onChange={(e) => updateField('monitorVMs', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Virtual Machines</span>
                          </label>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorContainers}
                              onChange={(e) => updateField('monitorContainers', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Containers</span>
                          </label>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorStorage}
                              onChange={(e) => updateField('monitorStorage', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Storage</span>
                          </label>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorBackups}
                              onChange={(e) => updateField('monitorBackups', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Backups</span>
                          </label>
                        </>
                      ) : (
                        <>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorDatastores}
                              onChange={(e) => updateField('monitorDatastores', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Datastores</span>
                          </label>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorSyncJobs}
                              onChange={(e) => updateField('monitorSyncJobs', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Sync Jobs</span>
                          </label>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorVerifyJobs}
                              onChange={(e) => updateField('monitorVerifyJobs', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Verify Jobs</span>
                          </label>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorPruneJobs}
                              onChange={(e) => updateField('monitorPruneJobs', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Prune Jobs</span>
                          </label>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorGarbageJobs}
                              onChange={(e) => updateField('monitorGarbageJobs', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Garbage Collection Jobs</span>
                          </label>
                        </>
                      )}
                    </div>
                  </div>
                </div>
                
                {/* Test Result */}
                <Show when={testResult()}>
                  <div class={`mx-6 p-3 rounded-lg text-sm ${
                    testResult()?.status === 'success' 
                      ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 text-green-800 dark:text-green-200'
                      : 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-800 dark:text-red-200'
                  }`}>
                    <div class="flex items-start gap-2">
                      <Show when={testResult()?.status === 'success'}>
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="flex-shrink-0 mt-0.5">
                          <path d="M9 12l2 2 4-4"></path>
                          <circle cx="12" cy="12" r="10"></circle>
                        </svg>
                      </Show>
                      <Show when={testResult()?.status === 'error'}>
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="flex-shrink-0 mt-0.5">
                          <circle cx="12" cy="12" r="10"></circle>
                          <line x1="15" y1="9" x2="9" y2="15"></line>
                          <line x1="9" y1="9" x2="15" y2="15"></line>
                        </svg>
                      </Show>
                      <div>
                        <p>{testResult()?.message}</p>
                        <Show when={testResult()?.isCluster}>
                          <p class="mt-1 text-xs opacity-80">✨ Cluster detected! All cluster nodes will be automatically added.</p>
                        </Show>
                      </div>
                    </div>
                  </div>
                </Show>
                
                {/* Footer */}
                <div class="flex items-center justify-between px-6 py-4 border-t border-gray-200 dark:border-gray-700">
                  <button
                    type="button"
                    onClick={handleTestConnection}
                    disabled={isTesting()}
                    class="px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {isTesting() ? 'Testing...' : 'Test Connection'}
                  </button>
                  
                  <div class="flex items-center gap-3">
                    <Show when={props.showBackToDiscovery && props.onBackToDiscovery}>
                      <button
                        type="button"
                        onClick={() => {
                          props.onBackToDiscovery!();
                          props.onClose();
                        }}
                        class="px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors flex items-center gap-2"
                      >
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <line x1="19" y1="12" x2="5" y2="12"></line>
                          <polyline points="12 19 5 12 12 5"></polyline>
                        </svg>
                        Back to Discovery
                      </button>
                    </Show>
                    <button
                      type="button"
                      onClick={props.onClose}
                      class="px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
                    >
                      Cancel
                    </button>
                    <button
                      type="submit"
                      class="px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                    >
                      {props.editingNode ? 'Update' : 'Add'} Node
                    </button>
                  </div>
                </div>
              </form>
            </div>
          </div>
        </div>
      </Show>
    </Portal>
  );
};