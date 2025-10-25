import { Component, Show, createSignal, createEffect } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { NodeConfig } from '@/types/nodes';
import type { SecurityStatus } from '@/types/config';
import { copyToClipboard } from '@/utils/clipboard';
import { showSuccess, showError } from '@/utils/toast';
import { NodesAPI } from '@/api/nodes';
import { SectionHeader } from '@/components/shared/SectionHeader';
import {
  formField,
  formHelpText,
  controlClass,
  labelClass,
  formCheckbox,
} from '@/components/shared/Form';

interface NodeModalProps {
  isOpen: boolean;
  resetKey?: number;
  onClose: () => void;
  nodeType: 'pve' | 'pbs' | 'pmg';
  editingNode?: NodeConfig;
  onSave: (nodeData: Partial<NodeConfig>) => void;
  showBackToDiscovery?: boolean;
  onBackToDiscovery?: () => void;
  securityStatus?: Partial<SecurityStatus>;
}

const deriveNameFromHost = (host: string): string => {
  let value = host.trim();
  if (!value) {
    return '';
  }

  try {
    const url = value.includes('://') ? new URL(value) : new URL(`https://${value}`);
    value = url.hostname || value;
  } catch {
    value = value.replace(/^https?:\/\//, '');
  }

  value = value.replace(/\/.*$/, '').replace(/^\[(.*)\]$/, '$1');
  value = value.replace(/\s+/g, '-');

  return value;
};

export const NodeModal: Component<NodeModalProps> = (props) => {
  const [testResult, setTestResult] = createSignal<{
    status: string;
    message: string;
    isCluster?: boolean;
  } | null>(null);
  const [isTesting, setIsTesting] = createSignal(false);

  // Function to get clean form data
  const getCleanFormData = (nodeType: 'pve' | 'pbs' | 'pmg' = props.nodeType) => ({
    name: '',
    host: '',
    authType: nodeType === 'pmg' ? 'password' : ('token' as 'password' | 'token'),
    setupMode: 'auto' as 'auto' | 'manual',
    user: '',
    password: '',
    tokenName: '',
    tokenValue: '',
    fingerprint: '',
    verifySSL: true,
    monitorPhysicalDisks: false,
    monitorMailStats: true,
    monitorQueues: true,
    monitorQuarantine: true,
    monitorDomainStats: false,
  });

  const [formData, setFormData] = createSignal(getCleanFormData());
  const [quickSetupCommand, setQuickSetupCommand] = createSignal('');
  const [quickSetupToken, setQuickSetupToken] = createSignal('');
  const [quickSetupExpiry, setQuickSetupExpiry] = createSignal<number | null>(null);
  const quickSetupExpiryLabel = () => {
    const expiry = quickSetupExpiry();
    if (!expiry) {
      return '';
    }
    try {
      return new Date(expiry * 1000).toLocaleTimeString();
    } catch {
      return '';
    }
  };

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
      setFormData(() => getCleanFormData(props.nodeType));
      setQuickSetupCommand('');
      setQuickSetupToken('');
      setQuickSetupExpiry(null);
      setTestResult(null);
      return;
    }

    // Force reset if node type changed
    if (nodeType !== previousNodeType && previousNodeType !== undefined) {
      previousNodeType = nodeType;
      setFormData(() => getCleanFormData(props.nodeType));
      setQuickSetupCommand('');
      setQuickSetupToken('');
      setQuickSetupExpiry(null);
      setTestResult(null);
      return;
    }
    previousNodeType = nodeType;

    // Reset when opening for new node
    if (isOpen && !editingNode) {
      setFormData(() => getCleanFormData(props.nodeType));
      setQuickSetupCommand('');
      setQuickSetupToken('');
      setQuickSetupExpiry(null);
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
      let username = ('user' in node ? node.user : '') || '';
      let tokenName = node.tokenName || '';

      const usesToken =
        node.type !== 'pve' && tokenName && tokenName.includes('!') && !node.hasPassword;
      if (usesToken) {
        const parts = tokenName.split('!');
        username = parts[0];
      }

      const pmgConfig =
        node.type === 'pmg'
          ? (node as NodeConfig & {
              monitorMailStats?: boolean;
              monitorQueues?: boolean;
              monitorQuarantine?: boolean;
              monitorDomainStats?: boolean;
            })
          : undefined;

      setFormData({
        name: node.name || '',
        host: node.host || '',
        authType: node.hasPassword ? 'password' : 'token',
        setupMode: 'auto',
        user: username,
        password: '',
        tokenName: tokenName,
        tokenValue: '',
        fingerprint: ('fingerprint' in node ? node.fingerprint : '') || '',
        verifySSL: node.verifySSL ?? true,
        monitorPhysicalDisks:
          node.type === 'pve'
            ? (node as NodeConfig & { monitorPhysicalDisks?: boolean }).monitorPhysicalDisks ?? true
            : false,
        monitorMailStats: pmgConfig?.monitorMailStats ?? true,
        monitorQueues: pmgConfig?.monitorQueues ?? true,
        monitorQuarantine: pmgConfig?.monitorQuarantine ?? true,
        monitorDomainStats: pmgConfig?.monitorDomainStats ?? false,
      });
    }
  });

  const handleSubmit = (e: Event) => {
    e.preventDefault();
    const data = formData();

    const normalizedName = data.name.trim() || deriveNameFromHost(data.host);
    if (!normalizedName) {
      showError('Node name is required');
      return;
    }

    if (normalizedName !== data.name) {
      setFormData((prev) => ({ ...prev, name: normalizedName }));
    }

    // Prepare data based on auth type
    const nodeData: Partial<NodeConfig> = {
      type: props.nodeType,
      name: normalizedName,
      host: data.host,
      fingerprint: data.fingerprint,
      verifySSL: data.verifySSL,
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
        monitorVMs: true,
        monitorContainers: true,
        monitorStorage: true,
        monitorBackups: true,
        monitorPhysicalDisks: data.monitorPhysicalDisks,
      });
    } else if (props.nodeType === 'pbs') {
      Object.assign(nodeData, {
        monitorDatastores: true,
        monitorSyncJobs: true,
        monitorVerifyJobs: true,
        monitorPruneJobs: true,
        monitorGarbageJobs: true,
      });
    } else {
      Object.assign(nodeData, {
        monitorMailStats: data.monitorMailStats,
        monitorQueues: data.monitorQueues,
        monitorQuarantine: data.monitorQuarantine,
        monitorDomainStats: data.monitorDomainStats,
      });
    }

    props.onSave(nodeData);
  };

  const updateField = (field: string, value: string | boolean) => {
    if (field === 'host' && typeof value === 'string') {
      setFormData((prev) => {
        const next = { ...prev, host: value };
        const derivedName = deriveNameFromHost(value);
        const previousDerivedName = deriveNameFromHost(prev.host || '');
        const shouldAutoUpdate =
          !prev.name.trim() || (previousDerivedName && prev.name === previousDerivedName);
        if (derivedName && shouldAutoUpdate) {
          next.name = derivedName;
        }
        return next;
      });
      setQuickSetupCommand('');
      return;
    }

    setFormData((prev) => ({ ...prev, [field]: value }));

    if (field === 'setupMode' && value !== 'auto') {
      setQuickSetupCommand('');
    }
  };

  const handleTestConnection = async () => {
    const data = formData();
    const normalizedName = data.name.trim() || deriveNameFromHost(data.host);

    if (!data.name.trim() && normalizedName) {
      setFormData((prev) => ({ ...prev, name: normalizedName }));
    }

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
            message: result.message || 'Connection successful',
          });
        } catch (error) {
          console.error('Test existing node error:', error);
          let errorMessage = 'Connection failed';
          if (error instanceof Error) {
            // Remove "API request failed: XXX " prefix if present
            errorMessage = error.message.replace(/^API request failed: \d{3}\s*/, '');
          }
          setTestResult({
            status: 'error',
            message: errorMessage,
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
      name: normalizedName || '',
      host: data.host,
      fingerprint: data.fingerprint,
      verifySSL: data.verifySSL,
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
        isCluster: result.isCluster,
      });
    } catch (error) {
      console.error('Test connection error:', error);
      let errorMessage = 'Connection failed';
      if (error instanceof Error) {
        // Remove "API request failed: XXX " prefix if present
        errorMessage = error.message.replace(/^API request failed: \d{3}\s*/, '');
      }
      setTestResult({
        status: 'error',
        message: errorMessage,
      });
    } finally {
      setIsTesting(false);
    }
  };

  const nodeProductName = () => {
    switch (props.nodeType) {
      case 'pve':
        return 'Proxmox VE';
      case 'pbs':
        return 'Proxmox Backup Server';
      default:
        return 'Proxmox Mail Gateway';
    }
  };

  return (
    <Portal>
      <Show when={props.isOpen}>
        <div class="fixed inset-0 z-50 overflow-y-auto">
          <div class="flex min-h-screen items-center justify-center p-4">
            {/* Backdrop */}
            <div class="fixed inset-0 bg-black/50 transition-opacity" onClick={props.onClose} />

            {/* Modal */}
            <div class="relative w-full max-w-2xl bg-white dark:bg-gray-800 rounded-lg shadow-xl">
              <form onSubmit={handleSubmit}>
                {/* Header */}
                <div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
                  <SectionHeader
                    title={`${props.editingNode ? 'Edit' : 'Add'} ${nodeProductName()} node`}
                    size="md"
                    class="flex-1"
                  />
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
                <div class="p-6 space-y-6">
                  {/* Basic Information */}
                  <div>
                    <SectionHeader
                      title="Basic information"
                      size="sm"
                      class="mb-4"
                      titleClass="text-gray-900 dark:text-gray-100"
                    />
                    <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
                      <div class={formField}>
                        <label class={labelClass('flex items-center gap-2')}>
                          Node Name <span class="text-red-500">*</span>
                        </label>
                        <input
                          type="text"
                          value={formData().name}
                          onInput={(e) => updateField('name', e.currentTarget.value)}
                          placeholder="Pulse uses this label across dashboards"
                          required
                          class={controlClass()}
                        />
                        <p class={formHelpText}>
                          Required and must be unique. We can auto-fill it from the Host URL if you leave it blank.
                        </p>
                      </div>

                      <div class={formField}>
                        <label class={labelClass('flex items-center gap-1')}>
                          Host URL <span class="text-red-500">*</span>
                        </label>
                        <input
                          type="text"
                          value={formData().host}
                          onInput={(e) => updateField('host', e.currentTarget.value)}
                          placeholder={
                            props.nodeType === 'pve'
                              ? 'https://proxmox.example.com:8006'
                              : props.nodeType === 'pbs'
                                ? 'https://backup.example.com:8007'
                                : 'https://mail-gateway.example.com:8006'
                          }
                          required
                          class={controlClass()}
                        />
                        <Show when={props.nodeType === 'pbs'}>
                          <p class={formHelpText}>
                            PBS requires HTTPS (not HTTP). Default port is 8007.
                          </p>
                        </Show>
                        <Show when={props.nodeType === 'pmg'}>
                          <p class={formHelpText}>
                            PMG API listens on HTTPS. Default port is 8006.
                          </p>
                        </Show>
                      </div>
                    </div>
                  </div>

                  {/* Authentication */}
                  <div>
                    <SectionHeader
                      title="Authentication"
                      size="sm"
                      class="mb-4"
                      titleClass="text-gray-900 dark:text-gray-100"
                    />

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
                          <span class="text-sm text-gray-700 dark:text-gray-300">
                            Username & Password
                          </span>
                        </label>
                        <Show when={props.nodeType !== 'pmg'}>
                          <label class="flex items-center">
                            <input
                              type="radio"
                              name="authType"
                              value="token"
                              checked={formData().authType === 'token'}
                              onChange={() => updateField('authType', 'token')}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">
                              API Token{' '}
                              <span class="text-green-600 dark:text-green-400 text-xs ml-1">
                                (Recommended)
                              </span>
                            </span>
                          </label>
                        </Show>
                      </div>
                      <Show when={props.nodeType === 'pmg'}>
                        <p class="text-xs text-gray-500 dark:text-gray-400 mt-2">
                          Proxmox Mail Gateway does not support API tokens. Use a service account with
                          password authentication (for example <code>root@pam</code> or a dedicated{' '}
                          <code>api@pmg</code> user).
                        </p>
                      </Show>
                    </div>

                    {/* Password Auth Fields */}
                    <Show when={formData().authType === 'password'}>
                      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
                        <div class={formField}>
                          <label class={labelClass()}>
                            Username <span class="text-red-500">*</span>
                          </label>
                          <input
                            type="text"
                            value={formData().user}
                            onInput={(e) => updateField('user', e.currentTarget.value)}
                            placeholder={
                              props.nodeType === 'pve'
                                ? 'root@pam'
                                : props.nodeType === 'pbs'
                                  ? 'admin@pbs'
                                  : 'root@pam'
                            }
                            required={formData().authType === 'password'}
                            class={controlClass()}
                          />
                          <Show when={props.nodeType === 'pbs'}>
                            <p class={formHelpText}>Must include realm (e.g., admin@pbs).</p>
                          </Show>
                          <Show when={props.nodeType === 'pmg'}>
                            <p class={formHelpText}>Include realm (e.g., root@pam or api@pmg).</p>
                          </Show>
                        </div>

                        <div class={formField}>
                          <label class={labelClass('flex items-center gap-2')}>
                            Password
                            <Show when={!props.editingNode}>
                              <span class="text-red-500">*</span>
                            </Show>
                          </label>
                          <input
                            type="password"
                            value={formData().password}
                            onInput={(e) => updateField('password', e.currentTarget.value)}
                            placeholder={
                              props.editingNode ? 'Leave blank to keep existing' : 'Password'
                            }
                            required={formData().authType === 'password' && !props.editingNode}
                            class={controlClass()}
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
                            <svg
                              width="16"
                              height="16"
                              viewBox="0 0 24 24"
                              fill="none"
                              stroke="currentColor"
                              stroke-width="2"
                            >
                              <circle cx="12" cy="12" r="10"></circle>
                              <path d="M12 6v6l4 2"></path>
                            </svg>
                            Quick Token Setup
                          </h5>

                          <Show when={props.nodeType === 'pve'}>
                            <div class="space-y-3 text-xs">
                              {/* Tab buttons */}
                              <div class="flex gap-2">
                                <button
                                  type="button"
                                  onClick={() => updateField('setupMode', 'auto')}
                                  class={`inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border border-transparent transition-colors ${
                                    formData().setupMode === 'auto'
                                      ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-300 border-gray-300 dark:border-gray-600 shadow-sm'
                                      : 'text-gray-600 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-300 hover:bg-gray-200/60 dark:hover:bg-gray-700/60'
                                  }`}
                                >
                                  Quick Setup
                                </button>
                                <button
                                  type="button"
                                  onClick={() => updateField('setupMode', 'manual')}
                                  class={`inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border border-transparent transition-colors ${
                                    formData().setupMode === 'manual'
                                      ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-300 border-gray-300 dark:border-gray-600 shadow-sm'
                                      : 'text-gray-600 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-300 hover:bg-gray-200/60 dark:hover:bg-gray-700/60'
                                  }`}
                                >
                                  Manual Setup
                                </button>
                              </div>

                              {/* Quick Setup Tab */}
                              <Show when={formData().setupMode === 'auto' || !formData().setupMode}>
                                <p class="text-xs text-gray-600 dark:text-gray-400 mb-3">
                                  The command below creates the monitoring user, applies read-only
                                  access, and adds the storage permissions Pulse needs to display
                                  backups.
                                </p>
                                <p class="text-blue-800 dark:text-blue-200">
                                  Just copy and run this one command on your Proxmox VE server:
                                </p>

                                {/* One-line command */}
                                <div class="space-y-3">
                                  <div class="relative bg-gray-900 rounded-md p-3 font-mono text-xs overflow-x-auto">
                                    <button
                                      type="button"
                                      onClick={async () => {
                                        console.log('[Quick Setup] Copy button clicked');
                                        try {
                                          // Check if host is populated
                                          if (!formData().host || formData().host.trim() === '') {
                                            console.log('[Quick Setup] No host entered');
                                            showError('Please enter the Host URL first');
                                            return;
                                          }

                                          console.log('[Quick Setup] Generating setup URL for host:', formData().host);
                                          // Always regenerate URL when host changes
                                          const { apiFetch } = await import('@/utils/apiClient');
                                          const response = await apiFetch('/api/setup-script-url', {
                                            method: 'POST',
                                            headers: { 'Content-Type': 'application/json' },
                                            body: JSON.stringify({
                                              type: 'pve',
                                              host: formData().host,
                                              backupPerms: true,
                                            }),
                                          });

                                          console.log('[Quick Setup] API response:', response.status, response.ok);
                                          if (response.ok) {
                                            const data = await response.json();
                                            console.log('[Quick Setup] Setup data received:', data);

                                            setQuickSetupToken(data.setupToken ?? '');
                                            setQuickSetupExpiry(typeof data.expires === 'number' ? data.expires : null);

                                            // Backend returns url, command, and expires
                                            // Just copy the command - don't show the modal
                                            if (data.command) {
                                              console.log('[Quick Setup] Copying command to clipboard:', data.command);
                                              setQuickSetupCommand(data.command);
                                              const copied = await copyToClipboard(data.command);
                                              console.log('[Quick Setup] Copy result:', copied);
                                              if (copied) {
                                                showSuccess('Command copied to clipboard! Paste the setup token shown below when prompted.');
                                              } else {
                                                showError('Failed to copy to clipboard');
                                              }
                                            } else {
                                              console.log('[Quick Setup] No command in response');
                                            }
                                          } else {
                                            setQuickSetupToken('');
                                            setQuickSetupExpiry(null);
                                            showError('Failed to generate setup URL');
                                          }
                                        } catch (error) {
                                          console.error('[Quick Setup] Error:', error);
                                          setQuickSetupToken('');
                                          setQuickSetupExpiry(null);
                                          showError('Failed to copy command');
                                        }
                                      }}
                                      class="absolute top-2 right-2 p-1.5 text-gray-400 hover:text-gray-200 bg-gray-700 rounded-md transition-colors"
                                      title="Copy command"
                                    >
                                      <svg
                                        width="16"
                                        height="16"
                                        viewBox="0 0 24 24"
                                        fill="none"
                                        stroke="currentColor"
                                        stroke-width="2"
                                      >
                                        <rect
                                          x="9"
                                          y="9"
                                          width="13"
                                          height="13"
                                          rx="2"
                                          ry="2"
                                        ></rect>
                                        <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                      </svg>
                                    </button>
                                    <Show
                                      when={quickSetupCommand().length > 0}
                                      fallback={
                                        <code class="text-blue-400">
                                          {formData().host
                                            ? 'Click the button above to copy the setup command'
                                            : '⚠️ Please enter the Host URL above first'}
                                        </code>
                                      }
                                    >
                                      <code class="block text-blue-100 whitespace-pre-wrap break-words">
                                        {quickSetupCommand()}
                                      </code>
                                    </Show>
                                    <Show when={quickSetupToken().length > 0}>
                                      <div class="mt-2 text-xs text-blue-800 dark:text-blue-200">
                                        <span class="font-semibold">Setup token:</span>
                                        <code class="ml-1 font-mono break-all text-blue-900 dark:text-blue-100">
                                          {quickSetupToken()}
                                        </code>
                                        <Show when={quickSetupExpiry()}>
                                          <span class="ml-2">
                                            Expires at {quickSetupExpiryLabel()}
                                          </span>
                                        </Show>
                                      </div>
                                    </Show>
                                  </div>

                                  <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-3">
                                    <div class="flex items-start space-x-2">
                                      <svg
                                        class="h-5 w-5 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0"
                                        fill="none"
                                        viewBox="0 0 24 24"
                                        stroke="currentColor"
                                      >
                                        <path
                                          stroke-linecap="round"
                                          stroke-linejoin="round"
                                          stroke-width="2"
                                          d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                                        />
                                      </svg>
                                      <div class="text-xs text-amber-700 dark:text-amber-300">
                                        <p class="font-semibold mb-1">
                                          If the command doesn't work:
                                        </p>
                                        <p>
                                          Your Proxmox server may not be able to reach Pulse. Use
                                          the alternative method below.
                                        </p>
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
                                            const pulseUrl = encodeURIComponent(
                                              window.location.origin,
                                            );
                                            const scriptUrl = `/api/setup-script?type=pve&host=${encodedHost}&pulse_url=${pulseUrl}&backup_perms=true`;

                                            // Fetch the script using the current session
                                            const response = await fetch(scriptUrl);
                                            if (!response.ok) {
                                              throw new Error('Failed to fetch setup script');
                                            }
                                            const scriptContent = await response.text();

                                            // Create a blob and download
                                            const blob = new Blob([scriptContent], {
                                              type: 'text/plain',
                                            });
                                            const url = URL.createObjectURL(blob);
                                            const a = document.createElement('a');
                                            a.href = url;
                                            a.download = 'pulse-setup.sh';
                                            document.body.appendChild(a);
                                            a.click();
                                            document.body.removeChild(a);
                                            URL.revokeObjectURL(url);

                                            showSuccess(
                                              'Script downloaded! Upload it to your server and run: bash pulse-setup.sh',
                                            );
                                          } catch (error) {
                                            console.error('Failed to download script:', error);
                                            showSuccess(
                                              'Failed to download script. Please check your connection.',
                                            );
                                          }
                                        }}
                                        class="w-full px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md transition-colors text-sm font-medium"
                                      >
                                        Download pulse-setup.sh
                                      </button>
                                      <div class="text-xs text-gray-600 dark:text-gray-400">
                                        1. Click to download the script
                                        <br />
                                        2. Upload to your server via SCP/SFTP
                                        <br />
                                        3. Run:{' '}
                                        <code class="bg-gray-100 dark:bg-gray-800 px-1 rounded">
                                          bash pulse-setup.sh
                                        </code>
                                      </div>
                                    </div>
                                  </details>
                                </div>

                                <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-3">
                                  <p class="text-sm font-semibold text-blue-800 dark:text-blue-200 mb-2">
                                    What this does:
                                  </p>
                                  <ul class="text-xs text-blue-700 dark:text-blue-300 space-y-1">
                                    <li class="flex items-start">
                                      <span class="text-green-500 mr-2 mt-0.5">✓</span>
                                      <span>
                                        Creates monitoring user{' '}
                                        <code class="bg-blue-100 dark:bg-blue-800/50 px-1 rounded">
                                          pulse-monitor@pam
                                        </code>
                                      </span>
                                    </li>
                                    <li class="flex items-start">
                                      <span class="text-green-500 mr-2 mt-0.5">✓</span>
                                      <span>Generates secure API token</span>
                                    </li>
                                    <li class="flex items-start">
                                      <span class="text-green-500 mr-2 mt-0.5">✓</span>
                                      <span>
                                        Sets up monitoring permissions (PVEAuditor + guest agent
                                        access + backup visibility)
                                      </span>
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
                                <p class="text-blue-800 dark:text-blue-200 mb-2">
                                  Run these commands one by one on your Proxmox VE server:
                                </p>

                                <div class="space-y-3">
                                  {/* Step 1: Create user */}
                                  <div>
                                    <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                      1. Create monitoring user:
                                    </p>
                                    <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            'pveum user add pulse-monitor@pam --comment "Pulse monitoring service"';
                                          if (await copyToClipboard(cmd)) {
                                            showSuccess('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                        title="Copy command"
                                      >
                                        <svg
                                          width="14"
                                          height="14"
                                          viewBox="0 0 24 24"
                                          fill="none"
                                          stroke="currentColor"
                                          stroke-width="2"
                                        >
                                          <rect
                                            x="9"
                                            y="9"
                                            width="13"
                                            height="13"
                                            rx="2"
                                            ry="2"
                                          ></rect>
                                          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                      </button>
                                      <code class="text-gray-800 dark:text-gray-200">
                                        pveum user add pulse-monitor@pam --comment "Pulse monitoring
                                        service"
                                      </code>
                                    </div>
                                  </div>

                                  {/* Step 2: Generate token */}
                                  <div>
                                    <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                      2. Generate API token (save the output!):
                                    </p>
                                    <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            'pveum user token add pulse-monitor@pam pulse-token --privsep 0';
                                          if (await copyToClipboard(cmd)) {
                                            showSuccess('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                        title="Copy command"
                                      >
                                        <svg
                                          width="14"
                                          height="14"
                                          viewBox="0 0 24 24"
                                          fill="none"
                                          stroke="currentColor"
                                          stroke-width="2"
                                        >
                                          <rect
                                            x="9"
                                            y="9"
                                            width="13"
                                            height="13"
                                            rx="2"
                                            ry="2"
                                          ></rect>
                                          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                      </button>
                                      <code class="text-gray-800 dark:text-gray-200">
                                        pveum user token add pulse-monitor@pam pulse-token --privsep
                                        0
                                      </code>
                                    </div>
                                    <p class="text-amber-600 dark:text-amber-400 text-xs mt-1">
                                      ⚠️ Copy the token value immediately - it won't be shown again!
                                    </p>
                                  </div>

                                  {/* Step 3: Set permissions */}
                                  <div>
                                    <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                      3. Set up monitoring permissions:
                                    </p>
                                    <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs mb-1">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            'pveum aclmod / -user pulse-monitor@pam -role PVEAuditor && if pveum role list 2>/dev/null | grep -q "VM.Monitor" || pveum role add TestMonitor -privs VM.Monitor 2>/dev/null; then pveum role delete TestMonitor 2>/dev/null; pveum role delete PulseMonitor 2>/dev/null; pveum role add PulseMonitor -privs VM.Monitor; pveum aclmod / -user pulse-monitor@pam -role PulseMonitor; fi';
                                          if (await copyToClipboard(cmd)) {
                                            showSuccess('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                        title="Copy command"
                                      >
                                        <svg
                                          width="14"
                                          height="14"
                                          viewBox="0 0 24 24"
                                          fill="none"
                                          stroke="currentColor"
                                          stroke-width="2"
                                        >
                                          <rect
                                            x="9"
                                            y="9"
                                            width="13"
                                            height="13"
                                            rx="2"
                                            ry="2"
                                          ></rect>
                                          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                      </button>
                                      <code class="text-gray-800 dark:text-gray-200 whitespace-pre-line">
                                        {
                                          '# Apply monitoring permissions - use built-in PVEAuditor role\npveum aclmod / -user pulse-monitor@pam -role PVEAuditor\n\n# Gather additional privileges for VM metrics\nEXTRA_PRIVS=()\n\n# Sys.Audit (Ceph, cluster status)\nif pveum role list 2>/dev/null | grep -q "Sys.Audit"; then\n  EXTRA_PRIVS+=(\"Sys.Audit\")\nelse\n  if pveum role add PulseTmpSysAudit -privs Sys.Audit 2>/dev/null; then\n    EXTRA_PRIVS+=(\"Sys.Audit\")\n    pveum role delete PulseTmpSysAudit 2>/dev/null\n  fi\nfi\n\n# VM guest agent / monitor privileges\nVM_PRIV=\"\"\nif pveum role list 2>/dev/null | grep -q "VM.Monitor"; then\n  VM_PRIV=\"VM.Monitor\"\nelif pveum role list 2>/dev/null | grep -q "VM.GuestAgent.Audit"; then\n  VM_PRIV=\"VM.GuestAgent.Audit\"\nelse\n  if pveum role add PulseTmpVMMonitor -privs VM.Monitor 2>/dev/null; then\n    VM_PRIV=\"VM.Monitor\"\n    pveum role delete PulseTmpVMMonitor 2>/dev/null\n  elif pveum role add PulseTmpGuestAudit -privs VM.GuestAgent.Audit 2>/dev/null; then\n    VM_PRIV=\"VM.GuestAgent.Audit\"\n    pveum role delete PulseTmpGuestAudit 2>/dev/null\n  fi\nfi\n\nif [ -n \"$VM_PRIV\" ]; then\n  EXTRA_PRIVS+=(\"$VM_PRIV\")\nfi\n\nif [ ${#EXTRA_PRIVS[@]} -gt 0 ]; then\n  PRIV_STRING=\"${EXTRA_PRIVS[*]}\"\n  pveum role delete PulseMonitor 2>/dev/null\n  pveum role add PulseMonitor -privs \"$PRIV_STRING\"\n  pveum aclmod / -user pulse-monitor@pam -role PulseMonitor\nfi'
                                        }
                                      </code>
                                    </div>
                                    <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            'pveum aclmod /storage -user pulse-monitor@pam -role PVEDatastoreAdmin';
                                          if (await copyToClipboard(cmd)) {
                                            showSuccess('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                        title="Copy command"
                                      >
                                        <svg
                                          width="14"
                                          height="14"
                                          viewBox="0 0 24 24"
                                          fill="none"
                                          stroke="currentColor"
                                          stroke-width="2"
                                        >
                                          <rect
                                            x="9"
                                            y="9"
                                            width="13"
                                            height="13"
                                            rx="2"
                                            ry="2"
                                          ></rect>
                                          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                      </button>
                                      <code class="text-gray-800 dark:text-gray-200">
                                        pveum aclmod /storage -user pulse-monitor@pam -role
                                        PVEDatastoreAdmin
                                      </code>
                                    </div>
                                    <p class="text-gray-600 dark:text-gray-400 text-xs mt-1">
                                      ℹ️ PVEAuditor gives read-only API access. PulseMonitor adds
                                      Sys.Audit plus either VM.Monitor (PVE 8) or VM.GuestAgent.Audit
                                      (PVE 9+) for disk and guest metrics. PVEDatastoreAdmin on
                                      /storage adds backup visibility.
                                    </p>
                                  </div>

                                  {/* Step 4: Use in Pulse */}
                                  <div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-md p-2">
                                    <p class="text-sm font-medium text-green-900 dark:text-green-100 mb-1">
                                      4. Add to Pulse with:
                                    </p>
                                    <ul class="text-xs text-green-800 dark:text-green-200 ml-4 list-disc">
                                      <li>
                                        <strong>Token ID:</strong> pulse-monitor@pam!pulse-token
                                      </li>
                                      <li>
                                        <strong>Token Value:</strong> [The value from step 2]
                                      </li>
                                      <li>
                                        <strong>Host URL:</strong>{' '}
                                        {formData().host || 'https://your-server:8006'}
                                      </li>
                                    </ul>
                                  </div>
                                </div>
                              </Show>
                            </div>
                          </Show>

                          <Show when={props.nodeType === 'pbs'}>
                            <div class="space-y-3 text-xs">
                              {/* Tab buttons for PBS */}
                              <div class="flex gap-2">
                                <button
                                  type="button"
                                  onClick={() => updateField('setupMode', 'auto')}
                                  class={`inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border border-transparent transition-colors ${
                                    formData().setupMode === 'auto' || !formData().setupMode
                                      ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-300 border-gray-300 dark:border-gray-600 shadow-sm'
                                      : 'text-gray-600 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-300 hover:bg-gray-200/60 dark:hover:bg-gray-700/60'
                                  }`}
                                >
                                  Quick Setup
                                </button>
                                <button
                                  type="button"
                                  onClick={() => updateField('setupMode', 'manual')}
                                  class={`inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border border-transparent transition-colors ${
                                    formData().setupMode === 'manual'
                                      ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-300 border-gray-300 dark:border-gray-600 shadow-sm'
                                      : 'text-gray-600 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-300 hover:bg-gray-200/60 dark:hover:bg-gray-700/60'
                                  }`}
                                >
                                  Manual Setup
                                </button>
                              </div>

                              {/* Quick Setup Tab for PBS */}
                              <Show when={formData().setupMode === 'auto' || !formData().setupMode}>
                                <p class="text-blue-800 dark:text-blue-200">
                                  Just copy and run this one command on your Proxmox Backup Server:
                                </p>

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
                                            const response = await apiFetch(
                                              '/api/setup-script-url',
                                              {
                                                method: 'POST',
                                                headers: { 'Content-Type': 'application/json' },
                                                body: JSON.stringify({
                                                  type: 'pbs',
                                                  host: formData().host,
                                                  backupPerms: false,
                                                }),
                                              },
                                            );

                                            if (response.ok) {
                                              const data = await response.json();
                                              if (data.command) {
                                                setQuickSetupCommand(data.command);
                                                if (await copyToClipboard(data.command)) {
                                                  showSuccess('Command copied to clipboard!');
                                                } else {
                                                  showError('Failed to copy to clipboard');
                                                }
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
                                        <svg
                                          width="16"
                                          height="16"
                                          viewBox="0 0 24 24"
                                          fill="none"
                                          stroke="currentColor"
                                          stroke-width="2"
                                        >
                                          <rect
                                            x="9"
                                            y="9"
                                            width="13"
                                            height="13"
                                            rx="2"
                                            ry="2"
                                          ></rect>
                                          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                      </button>
                                    </Show>
                                    <Show
                                      when={quickSetupCommand().length > 0}
                                      fallback={
                                        <code class="text-blue-400">
                                          {formData().host
                                            ? 'Click the button above to copy the setup command'
                                            : '⚠️ Please enter the Host URL above first'}
                                        </code>
                                      }
                                    >
                                      <code class="block text-blue-100 whitespace-pre-wrap break-words">
                                        {quickSetupCommand()}
                                      </code>
                                    </Show>
                                    <Show when={quickSetupToken().length > 0}>
                                      <div class="mt-2 text-xs text-blue-800 dark:text-blue-200">
                                        <span class="font-semibold">Setup token:</span>
                                        <code class="ml-1 font-mono break-all text-blue-900 dark:text-blue-100">
                                          {quickSetupToken()}
                                        </code>
                                        <Show when={quickSetupExpiry()}>
                                          <span class="ml-2">
                                            Expires at {quickSetupExpiryLabel()}
                                          </span>
                                        </Show>
                                      </div>
                                    </Show>
                                  </div>

                                  <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-3">
                                    <div class="flex items-start space-x-2">
                                      <svg
                                        class="h-5 w-5 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0"
                                        fill="none"
                                        viewBox="0 0 24 24"
                                        stroke="currentColor"
                                      >
                                        <path
                                          stroke-linecap="round"
                                          stroke-linejoin="round"
                                          stroke-width="2"
                                          d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                                        />
                                      </svg>
                                      <div class="text-xs text-amber-700 dark:text-amber-300">
                                        <p class="font-semibold mb-1">
                                          If the command doesn't work:
                                        </p>
                                        <p>
                                          Your PBS server may not be able to reach Pulse. Use the
                                          alternative method below.
                                        </p>
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
                                            const pulseUrl = encodeURIComponent(
                                              window.location.origin,
                                            );
                                            const scriptUrl = `/api/setup-script?type=pbs&host=${encodedHost}&pulse_url=${pulseUrl}`;

                                            // Fetch the script using the current session
                                            const response = await fetch(scriptUrl);
                                            if (!response.ok) {
                                              throw new Error('Failed to fetch setup script');
                                            }
                                            const scriptContent = await response.text();

                                            // Create a blob and download
                                            const blob = new Blob([scriptContent], {
                                              type: 'text/plain',
                                            });
                                            const url = URL.createObjectURL(blob);
                                            const a = document.createElement('a');
                                            a.href = url;
                                            a.download = 'pulse-pbs-setup.sh';
                                            document.body.appendChild(a);
                                            a.click();
                                            document.body.removeChild(a);
                                            URL.revokeObjectURL(url);

                                            showSuccess(
                                              'Script downloaded! Upload it to your PBS and run: bash pulse-pbs-setup.sh',
                                            );
                                          } catch (error) {
                                            console.error('Failed to download script:', error);
                                            showSuccess(
                                              'Failed to download script. Please check your connection.',
                                            );
                                          }
                                        }}
                                        class="w-full px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md transition-colors text-sm font-medium"
                                      >
                                        Download pulse-pbs-setup.sh
                                      </button>
                                      <div class="text-xs text-gray-600 dark:text-gray-400">
                                        1. Click to download the script
                                        <br />
                                        2. Upload to your PBS via SCP/SFTP
                                        <br />
                                        3. Run:{' '}
                                        <code class="bg-gray-100 dark:bg-gray-800 px-1 rounded">
                                          bash pulse-pbs-setup.sh
                                        </code>
                                      </div>
                                    </div>
                                  </details>
                                </div>

                                <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-3">
                                  <p class="text-sm font-semibold text-blue-800 dark:text-blue-200 mb-2">
                                    What this does:
                                  </p>
                                  <ul class="text-xs text-blue-700 dark:text-blue-300 space-y-1">
                                    <li class="flex items-start">
                                      <span class="text-green-500 mr-2 mt-0.5">✓</span>
                                      <span>
                                        Creates monitoring user{' '}
                                        <code class="bg-blue-100 dark:bg-blue-800/50 px-1 rounded">
                                          pulse-monitor@pbs
                                        </code>
                                      </span>
                                    </li>
                                    <li class="flex items-start">
                                      <span class="text-green-500 mr-2 mt-0.5">✓</span>
                                      <span>Generates secure API token</span>
                                    </li>
                                    <li class="flex items-start">
                                      <span class="text-green-500 mr-2 mt-0.5">✓</span>
                                      <span>
                                        Sets up Audit permissions (read-only access to backups +
                                        system stats)
                                      </span>
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
                                <p class="text-blue-800 dark:text-blue-200 mb-2">
                                  Run these commands one by one on your Proxmox Backup Server:
                                </p>

                                <div class="space-y-3">
                                  {/* Step 1: Create user */}
                                  <div>
                                    <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                      1. Create monitoring user:
                                    </p>
                                    <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            'proxmox-backup-manager user create pulse-monitor@pbs';
                                          if (await copyToClipboard(cmd)) {
                                            showSuccess('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                        title="Copy command"
                                      >
                                        <svg
                                          width="14"
                                          height="14"
                                          viewBox="0 0 24 24"
                                          fill="none"
                                          stroke="currentColor"
                                          stroke-width="2"
                                        >
                                          <rect
                                            x="9"
                                            y="9"
                                            width="13"
                                            height="13"
                                            rx="2"
                                            ry="2"
                                          ></rect>
                                          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                      </button>
                                      <code class="text-gray-800 dark:text-gray-200">
                                        proxmox-backup-manager user create pulse-monitor@pbs
                                      </code>
                                    </div>
                                  </div>

                                  {/* Step 2: Generate token */}
                                  <div>
                                    <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                      2. Generate API token (save the output!):
                                    </p>
                                    <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            'proxmox-backup-manager user generate-token pulse-monitor@pbs pulse-token';
                                          if (await copyToClipboard(cmd)) {
                                            showSuccess('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                        title="Copy command"
                                      >
                                        <svg
                                          width="14"
                                          height="14"
                                          viewBox="0 0 24 24"
                                          fill="none"
                                          stroke="currentColor"
                                          stroke-width="2"
                                        >
                                          <rect
                                            x="9"
                                            y="9"
                                            width="13"
                                            height="13"
                                            rx="2"
                                            ry="2"
                                          ></rect>
                                          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                      </button>
                                      <code class="text-gray-800 dark:text-gray-200">
                                        proxmox-backup-manager user generate-token pulse-monitor@pbs
                                        pulse-token
                                      </code>
                                    </div>
                                    <p class="text-amber-600 dark:text-amber-400 text-xs mt-1">
                                      ⚠️ Copy the token value immediately - it won't be shown again!
                                    </p>
                                  </div>

                                  {/* Step 3: Set permissions */}
                                  <div>
                                    <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                      3. Set up read-only permissions (includes system stats):
                                    </p>
                                    <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs mb-1">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            'proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs';
                                          if (await copyToClipboard(cmd)) {
                                            showSuccess('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                        title="Copy command"
                                      >
                                        <svg
                                          width="14"
                                          height="14"
                                          viewBox="0 0 24 24"
                                          fill="none"
                                          stroke="currentColor"
                                          stroke-width="2"
                                        >
                                          <rect
                                            x="9"
                                            y="9"
                                            width="13"
                                            height="13"
                                            rx="2"
                                            ry="2"
                                          ></rect>
                                          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                      </button>
                                      <code class="text-gray-800 dark:text-gray-200">
                                        proxmox-backup-manager acl update / Audit --auth-id
                                        pulse-monitor@pbs
                                      </code>
                                    </div>
                                    <div class="relative bg-white dark:bg-gray-800 rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            "proxmox-backup-manager acl update / Audit --auth-id 'pulse-monitor@pbs!pulse-token'";
                                          if (await copyToClipboard(cmd)) {
                                            showSuccess('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                        title="Copy command"
                                      >
                                        <svg
                                          width="14"
                                          height="14"
                                          viewBox="0 0 24 24"
                                          fill="none"
                                          stroke="currentColor"
                                          stroke-width="2"
                                        >
                                          <rect
                                            x="9"
                                            y="9"
                                            width="13"
                                            height="13"
                                            rx="2"
                                            ry="2"
                                          ></rect>
                                          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                        </svg>
                                      </button>
                                      <code class="text-gray-800 dark:text-gray-200">
                                        proxmox-backup-manager acl update / Audit --auth-id
                                        'pulse-monitor@pbs!pulse-token'
                                      </code>
                                    </div>
                                  </div>

                                  {/* Step 4: Use in Pulse */}
                                  <div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-md p-2">
                                    <p class="text-sm font-medium text-green-900 dark:text-green-100 mb-1">
                                      4. Add to Pulse with:
                                    </p>
                                    <ul class="text-xs text-green-800 dark:text-green-200 ml-4 list-disc">
                                      <li>
                                        <strong>Token ID:</strong> pulse-monitor@pbs!pulse-token
                                      </li>
                                      <li>
                                        <strong>Token Value:</strong> [The value from step 2]
                                      </li>
                                      <li>
                                        <strong>Host URL:</strong>{' '}
                                        {formData().host || 'https://your-server:8007'}
                                      </li>
                                    </ul>
                                  </div>

                                  {/* Permission Info Box */}
                                  <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-md p-2 mt-3">
                                    <p class="text-xs font-semibold text-amber-800 dark:text-amber-200 mb-1">
                                      About PBS Permissions:
                                    </p>
                                    <ul class="text-xs text-amber-700 dark:text-amber-300 space-y-0.5">
                                      <li>
                                        <strong>Basic (DatastoreAudit):</strong> View backups only
                                      </li>
                                      <li>
                                        <strong>Enhanced (Audit on /):</strong> View backups +
                                        CPU/memory/uptime stats
                                      </li>
                                      <li class="text-amber-600 dark:text-amber-400">
                                        → We use Enhanced for better monitoring visibility
                                      </li>
                                    </ul>
                                  </div>
                                </div>
                          </Show>
                        </div>
                      </Show>
                      <Show when={props.nodeType === 'pmg'}>
                        <div class="space-y-3 text-xs text-gray-700 dark:text-gray-200">
                          <p>
                            Generate a dedicated API token in <strong>Configuration → API Tokens</strong> on your
                            Mail Gateway. We recommend creating a service user such as <code class="font-mono">pulse-monitor@pmg</code>
                            with <em>Auditor</em> privileges.
                          </p>
                          <ol class="list-decimal ml-4 space-y-1">
                            <li>Click <em>Add</em> and choose the service user (or create one if needed).</li>
                            <li>Enable <em>Privilege Separation</em> and assign the <em>Auditor</em> role.</li>
                            <li>Copy the generated Token ID (e.g. <code class="font-mono">pulse-monitor@pmg!pulse-edge</code>) and the secret value into the fields below.</li>
                          </ol>
                          <p class="text-xs text-gray-500 dark:text-gray-400">
                            Pulse only requires read-only access. Avoid granting administrator permissions to the token.
                          </p>
                        </div>
                      </Show>
                    </div>

                    {/* Token Input Fields */}
                    <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
                          <div class={formField}>
                            <label class={labelClass()}>
                              Token ID <span class="text-red-500">*</span>
                            </label>
                            <input
                              type="text"
                              value={formData().tokenName}
                              onInput={(e) => updateField('tokenName', e.currentTarget.value)}
                              placeholder={
                                props.nodeType === 'pve'
                                  ? 'pulse-monitor@pam!pulse-token'
                                  : 'pulse-monitor@pbs!pulse-token'
                              }
                              required={formData().authType === 'token'}
                              class={controlClass('font-mono')}
                            />
                            <p class={formHelpText}>
                              Full token ID from Proxmox (user@realm!tokenname).
                            </p>
                          </div>

                          <div class={formField}>
                            <label class={labelClass('flex items-center gap-2')}>
                              Token Value
                              <Show when={!props.editingNode}>
                                <span class="text-red-500">*</span>
                              </Show>
                            </label>
                            <input
                              type="password"
                              value={formData().tokenValue}
                              onInput={(e) => updateField('tokenValue', e.currentTarget.value)}
                              placeholder={
                                props.editingNode
                                  ? 'Leave blank to keep existing'
                                  : 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'
                              }
                              required={formData().authType === 'token' && !props.editingNode}
                              class={controlClass('font-mono')}
                            />
                            <p class={formHelpText}>
                              The secret value shown when creating the token.
                            </p>
                          </div>
                        </div>
                      </div>
                    </Show>
                  </div>

                  {/* SSL Settings */}
                  <div>
                    <SectionHeader
                      title="SSL settings"
                      size="sm"
                      class="mb-4"
                      titleClass="text-gray-900 dark:text-gray-100"
                    />
                    <div class="space-y-3">
                      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                        <input
                          type="checkbox"
                          checked={formData().verifySSL}
                          onChange={(e) => updateField('verifySSL', e.currentTarget.checked)}
                          class={formCheckbox}
                        />
                        Verify SSL certificate
                      </label>

                      <div class={formField}>
                        <label class={labelClass()}>SSL Fingerprint (optional)</label>
                        <input
                          type="text"
                          value={formData().fingerprint}
                          onInput={(e) => updateField('fingerprint', e.currentTarget.value)}
                          placeholder="AA:BB:CC:DD:EE:FF:..."
                          class={controlClass('font-mono')}
                        />
                        <p class={formHelpText}>
                          Useful when connecting to servers with self-signed certificates.
                        </p>
                      </div>
                    </div>
                  </div>

                  {/* Monitoring Overview */}
                  <div>
                    <SectionHeader
                      title="Monitoring coverage"
                      size="sm"
                      class="mb-2"
                      titleClass="text-gray-900 dark:text-gray-100"
                    />
                    <p class="text-sm text-gray-600 dark:text-gray-400">
                      {props.nodeType === 'pmg'
                        ? 'Pulse captures mail flow analytics, rejection causes, and quarantine visibility without additional scripts.'
                        : 'Pulse automatically tracks all supported resources for this node — virtual machines, containers, storage usage, backups, and PBS job activity — so you always get full visibility without extra configuration.'}
                    </p>
                  </div>

                  {/* Physical Disk Monitoring - PVE only */}
                  <Show when={props.nodeType === 'pve'}>
                    <div>
                      <SectionHeader
                        title="Advanced monitoring"
                        size="sm"
                        class="mb-3"
                        titleClass="text-gray-900 dark:text-gray-100"
                      />
                      <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300">
                        <input
                          type="checkbox"
                          checked={formData().monitorPhysicalDisks}
                          onChange={(e) => updateField('monitorPhysicalDisks', e.currentTarget.checked)}
                          class={formCheckbox + ' mt-0.5'}
                        />
                        <div>
                          <div>Monitor physical disk health (SMART)</div>
                          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                            Polls disk SMART data every 5 minutes. Note: This will cause HDDs to spin up from standby.
                            If you have HDDs that should stay idle, leave this disabled.
                          </p>
                        </div>
                      </label>
                    </div>
                  </Show>
                  <Show when={props.nodeType === 'pmg'}>
                    <div class="space-y-3">
                      <SectionHeader
                        title="Data collection"
                        size="sm"
                        class="mb-1"
                        titleClass="text-gray-900 dark:text-gray-100"
                      />
                      <p class="text-xs text-gray-600 dark:text-gray-400">
                        Control which PMG data sets Pulse ingests. Disable individual collectors if you want to limit API usage.
                      </p>

                      <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300">
                        <input
                          type="checkbox"
                          checked={formData().monitorMailStats}
                          onChange={(e) => updateField('monitorMailStats', e.currentTarget.checked)}
                          class={formCheckbox + ' mt-0.5'}
                        />
                        <div>
                          <div>Mail statistics &amp; trends</div>
                          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                            Total mail volume, inbound/outbound breakdown, spam and virus counts.
                          </p>
                        </div>
                      </label>

                      <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300">
                        <input
                          type="checkbox"
                          checked={formData().monitorQueues}
                          onChange={(e) => updateField('monitorQueues', e.currentTarget.checked)}
                          class={formCheckbox + ' mt-0.5'}
                        />
                        <div>
                          <div>Queue health insights</div>
                          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                            Track Postfix queue depth and rejection trends to spot delivery bottlenecks.
                          </p>
                        </div>
                      </label>

                      <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300">
                        <input
                          type="checkbox"
                          checked={formData().monitorQuarantine}
                          onChange={(e) => updateField('monitorQuarantine', e.currentTarget.checked)}
                          class={formCheckbox + ' mt-0.5'}
                        />
                        <div>
                          <div>Quarantine totals</div>
                          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                            Mirror PMG quarantine sizes for spam, virus, and attachment buckets.
                          </p>
                        </div>
                      </label>

                      <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300">
                        <input
                          type="checkbox"
                          checked={formData().monitorDomainStats}
                          onChange={(e) => updateField('monitorDomainStats', e.currentTarget.checked)}
                          class={formCheckbox + ' mt-0.5'}
                        />
                        <div>
                          <div>Domain-level statistics</div>
                          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                            Gather per-domain metrics for deeper mail routing analysis.
                          </p>
                        </div>
                      </label>
                    </div>
                  </Show>
                </div>

                {/* Test Result */}
                <Show when={testResult()}>
                  {(() => {
                    const result = testResult();
                    console.log('Test result display:', {
                      status: result?.status,
                      message: result?.message,
                    });
                    return null;
                  })()}
                  <div
                    class={`mx-6 p-3 rounded-lg text-sm ${
                      testResult()?.status === 'success'
                        ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 text-green-800 dark:text-green-200'
                        : 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-800 dark:text-red-200'
                    }`}
                  >
                    <div class="flex items-start gap-2">
                      <Show when={testResult()?.status === 'success'}>
                        <svg
                          width="16"
                          height="16"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          stroke-width="2"
                          class="flex-shrink-0 mt-0.5"
                        >
                          <path d="M9 12l2 2 4-4"></path>
                          <circle cx="12" cy="12" r="10"></circle>
                        </svg>
                      </Show>
                      <Show when={testResult()?.status === 'error'}>
                        <svg
                          width="16"
                          height="16"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          stroke-width="2"
                          class="flex-shrink-0 mt-0.5"
                        >
                          <circle cx="12" cy="12" r="10"></circle>
                          <line x1="15" y1="9" x2="9" y2="15"></line>
                          <line x1="9" y1="9" x2="15" y2="15"></line>
                        </svg>
                      </Show>
                      <div>
                        <p>{testResult()?.message}</p>
                        <Show when={testResult()?.isCluster}>
                          <p class="mt-1 text-xs opacity-80">
                            ✨ Cluster detected! All cluster nodes will be automatically added.
                          </p>
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
                        <svg
                          width="16"
                          height="16"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          stroke-width="2"
                        >
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
