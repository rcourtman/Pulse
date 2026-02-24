import { Component, Show, For, createSignal, createEffect } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { NodeConfig } from '@/types/nodes';
import type { SecurityStatus } from '@/types/config';
import { copyToClipboard } from '@/utils/clipboard';

import { notificationStore } from '@/stores/notifications';
import { getPulseBaseUrl } from '@/utils/url';
import { apiFetch } from '@/utils/apiClient';
import { NodesAPI } from '@/api/nodes';
import { SectionHeader } from '@/components/shared/SectionHeader';
import {
  formField,
  formHelpText,
  controlClass,
  labelClass,
  formCheckbox,
} from '@/components/shared/Form';
import { logger } from '@/utils/logger';
import { TogglePrimitive } from '@/components/shared/Toggle';

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
  temperatureMonitoringEnabled?: boolean;
  temperatureMonitoringLocked?: boolean;
  savingTemperatureSetting?: boolean;
  onToggleTemperatureMonitoring?: (enabled: boolean) => Promise<void> | void;
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
    warnings?: string[];
  } | null>(null);
  const [isTesting, setIsTesting] = createSignal(false);

  // Function to get clean form data
  const getCleanFormData = (nodeType: 'pve' | 'pbs' | 'pmg' = props.nodeType) => ({
    name: '',
    host: '',
    guestURL: '',
    authType: nodeType === 'pmg' ? 'password' : ('token' as 'password' | 'token'),
    setupMode: 'agent' as 'agent' | 'auto' | 'manual', // Default to agent install (recommended)
    user: '',
    password: '',
    tokenName: '',
    tokenValue: '',
    fingerprint: '',
    verifySSL: true,
    monitorPhysicalDisks: false,
    physicalDiskPollingMinutes: 5,
    monitorMailStats: true,
    monitorQueues: true,
    monitorQuarantine: true,
    monitorDomainStats: false,
  });

  const [formData, setFormData] = createSignal(getCleanFormData());
  const [quickSetupCommand, setQuickSetupCommand] = createSignal('');
  const [quickSetupToken, setQuickSetupToken] = createSignal('');
  const [quickSetupExpiry, setQuickSetupExpiry] = createSignal<number | null>(null);
  const [agentInstallCommand, setAgentInstallCommand] = createSignal('');
  const [loadingAgentCommand, setLoadingAgentCommand] = createSignal(false);
  const [agentCommandError, setAgentCommandError] = createSignal<string | null>(null);
  const isAdvancedSetupMode = () =>
    formData().setupMode === 'auto' || formData().setupMode === 'manual';
  const showTemperatureMonitoringSection = () =>
    typeof props.temperatureMonitoringEnabled === 'boolean';
  const temperatureMonitoringEnabledValue = () => props.temperatureMonitoringEnabled ?? true;
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
  let previousFormSourceSignature: string | null = null;

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
      previousFormSourceSignature = null;
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
      previousFormSourceSignature = null;
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
      previousFormSourceSignature = null;
    }
  });

  // Generate setup URL when Quick Setup tab is shown
  // Skip auto-generation to avoid race conditions - generate on-demand when copy is clicked

  // Update form when editing node changes
  createEffect(() => {
    // Only populate form if we have an editing node AND it matches the current node type
    // This prevents PVE data from being used when adding a PBS node
    const node = props.editingNode;
    if (!node || node.type !== props.nodeType) {
      previousFormSourceSignature = null;
      return;
    }

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

    const formSource: ReturnType<typeof getCleanFormData> = {
      name: node.name || '',
      host: node.host || '',
      guestURL: ('guestURL' in node ? node.guestURL : '') || '',
      authType: node.hasPassword ? 'password' : 'token',
      setupMode: node.source === 'agent' ? 'agent' : 'auto',
      user: username,
      password: '',
      tokenName: tokenName,
      tokenValue: '',
      fingerprint: ('fingerprint' in node ? node.fingerprint : '') || '',
      verifySSL: node.verifySSL ?? true,
      monitorPhysicalDisks:
        node.type === 'pve'
          ? ((node as NodeConfig & { monitorPhysicalDisks?: boolean }).monitorPhysicalDisks ?? true)
          : false,
      physicalDiskPollingMinutes:
        node.type === 'pve'
          ? ((node as NodeConfig & { physicalDiskPollingMinutes?: number })
              .physicalDiskPollingMinutes ?? 5)
          : 5,
      monitorMailStats: pmgConfig?.monitorMailStats ?? true,
      monitorQueues: pmgConfig?.monitorQueues ?? true,
      monitorQuarantine: pmgConfig?.monitorQuarantine ?? true,
      monitorDomainStats: pmgConfig?.monitorDomainStats ?? false,
    };

    const formSourceSignature = JSON.stringify(formSource);
    if (formSourceSignature === previousFormSourceSignature) {
      return;
    }

    previousFormSourceSignature = formSourceSignature;
    setFormData(formSource);
  });

  const handleSubmit = (e: Event) => {
    e.preventDefault();
    const data = formData();

    const normalizedName = data.name.trim() || deriveNameFromHost(data.host);
    if (!normalizedName) {
      notificationStore.error('Node name is required');
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
      guestURL: data.guestURL,
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
        physicalDiskPollingMinutes: data.physicalDiskPollingMinutes,
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

  const updateField = (field: string, value: string | boolean | number) => {
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

    if (field === 'setupMode') {
      if (value !== 'auto') {
        setQuickSetupCommand('');
      }
      if (value !== 'agent') {
        setAgentInstallCommand('');
      }
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
          logger.error('Test existing node error:', error);
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
        status: result.warnings && result.warnings.length > 0 ? 'warning' : 'success',
        message: result.message || 'Connection successful',
        isCluster: result.isCluster,
        warnings: result.warnings,
      });
    } catch (error) {
      logger.error('Test connection error:', error);
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
            <div class="fixed inset-0 bg-black transition-opacity" onClick={props.onClose} />

            {/* Modal */}
            <div class="relative w-full max-w-2xl bg-surface rounded-md shadow-sm">
              <form onSubmit={handleSubmit}>
                {/* Header */}
                <div class="flex items-center justify-between p-4 border-b border-border">
                  <SectionHeader
                    title={`${props.editingNode ? 'Edit' : 'Add'} ${nodeProductName()} node`}
                    size="md"
                    class="flex-1"
                  />
                  <button
                    type="button"
                    onClick={props.onClose}
                    class="text-slate-400 hover:text-muted"
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
                      titleClass="text-base-content"
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
                          Required and must be unique. We can auto-fill it from the Host URL if you
                          leave it blank.
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

                      <div class={formField}>
                        <label class={labelClass('flex items-center gap-1')}>
                          Guest URL{' '}
                          <span class="text-slate-500 text-xs font-normal">(Optional)</span>
                        </label>
                        <input
                          type="text"
                          value={formData().guestURL}
                          onInput={(e) => updateField('guestURL', e.currentTarget.value)}
                          placeholder={
                            props.nodeType === 'pve'
                              ? 'https://pve.yourdomain.com'
                              : props.nodeType === 'pbs'
                                ? 'https://pbs.yourdomain.com'
                                : 'https://pmg.yourdomain.com'
                          }
                          class={controlClass()}
                        />
                        <p class={formHelpText}>
                          Optional guest-accessible URL for navigation. If specified, this URL will
                          be used when opening the web UI instead of the Host URL.
                        </p>
                      </div>
                    </div>
                  </div>

                  {/* Authentication */}
                  <div>
                    <SectionHeader
                      title="Authentication"
                      size="sm"
                      class="mb-4"
                      titleClass="text-base-content"
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
                          <span class="text-sm text-base-content">Username & Password</span>
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
                            <span class="text-sm text-base-content">
                              API Token{' '}
                              <span class="text-green-600 dark:text-green-400 text-xs ml-1">
                                (Recommended)
                              </span>
                            </span>
                          </label>
                        </Show>
                      </div>
                      <Show when={props.nodeType === 'pmg'}>
                        <p class="text-xs text-muted mt-2">
                          Proxmox Mail Gateway does not support API tokens. Use a service account
                          with password authentication (for example <code>root@pam</code> or a
                          dedicated <code>api@pmg</code> user).
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
                        <div class="bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 rounded-md p-4">
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
                              <div class="flex gap-2 flex-wrap">
                                <button
                                  type="button"
                                  onClick={() => updateField('setupMode', 'agent')}
                                  class={`inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border border-transparent transition-colors ${
                                    formData().setupMode === 'agent'
                                      ? 'bg-surface text-blue-600 dark:text-blue-300 border-border shadow-sm'
                                      : 'text-muted hover:text-blue-600 dark:hover:text-blue-300 hover:bg-surface-hover'
                                  }`}
                                >
                                  Agent Install
                                  <span class="ml-1.5 px-1.5 py-0.5 text-[10px] font-semibold bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">
                                    Recommended
                                  </span>
                                </button>
                                <button
                                  type="button"
                                  onClick={() => {
                                    if (formData().setupMode === 'agent') {
                                      updateField('setupMode', 'auto');
                                    }
                                  }}
                                  class={`inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border border-transparent transition-colors ${
                                    isAdvancedSetupMode()
                                      ? 'bg-surface text-blue-600 dark:text-blue-300 border-border shadow-sm'
                                      : 'text-muted hover:text-blue-600 dark:hover:text-blue-300 hover:bg-surface-hover'
                                  }`}
                                >
                                  Advanced
                                </button>
                              </div>

                              <Show when={isAdvancedSetupMode()}>
                                <div class="mt-1 flex gap-2 flex-wrap pl-0.5">
                                  <button
                                    type="button"
                                    onClick={() => updateField('setupMode', 'auto')}
                                    class={`inline-flex items-center px-2.5 py-1 text-xs font-medium rounded-md border border-transparent transition-colors ${
                                      formData().setupMode === 'auto'
                                        ? 'bg-surface text-blue-600 dark:text-blue-300 border-border shadow-sm'
                                        : 'text-muted hover:text-blue-600 dark:hover:text-blue-300 hover:bg-surface-hover'
                                    }`}
                                  >
                                    API Only
                                  </button>
                                  <button
                                    type="button"
                                    onClick={() => updateField('setupMode', 'manual')}
                                    class={`inline-flex items-center px-2.5 py-1 text-xs font-medium rounded-md border border-transparent transition-colors ${
                                      formData().setupMode === 'manual'
                                        ? 'bg-surface text-blue-600 dark:text-blue-300 border-border shadow-sm'
                                        : 'text-muted hover:text-blue-600 dark:hover:text-blue-300 hover:bg-surface-hover'
                                    }`}
                                  >
                                    Manual
                                  </button>
                                </div>
                              </Show>

                              {/* Agent Install Tab (Recommended) */}
                              <Show when={formData().setupMode === 'agent'}>
                                <div class="space-y-3">
                                  <p class="text-xs text-muted">
                                    Install the Pulse agent on your Proxmox node. This single
                                    command sets everything up:
                                  </p>
                                  <ul class="text-xs text-muted list-disc list-inside space-y-1">
                                    <li>Creates monitoring user and API token automatically</li>
                                    <li>Registers the node with Pulse</li>
                                    <li>Enables temperature monitoring (no SSH required)</li>
                                    <li>
                                      Enables Pulse Patrol automation for managing VMs/containers
                                    </li>
                                  </ul>
                                  <p class="text-blue-800 dark:text-blue-200 font-medium">
                                    Run this command on your Proxmox VE node:
                                  </p>
                                  <div class="relative bg-base rounded-md p-3 font-mono text-xs overflow-x-auto">
                                    <button
                                      type="button"
                                      disabled={loadingAgentCommand()}
                                      onClick={async () => {
                                        logger.debug('[Agent Install] Copy button clicked');
                                        try {
                                          setLoadingAgentCommand(true);
                                          const data = await NodesAPI.getAgentInstallCommand({
                                            type: 'pve',
                                            enableProxmox: true,
                                          });

                                          if (data.command) {
                                            setAgentInstallCommand(data.command);
                                            const copied = await copyToClipboard(data.command);
                                            if (copied) {
                                              notificationStore.success(
                                                'Command copied! Run it on your Proxmox node.',
                                              );
                                            } else {
                                              notificationStore.error(
                                                'Failed to copy to clipboard',
                                              );
                                            }
                                          }
                                        } catch (error) {
                                          logger.error('[Agent Install] Error:', error);
                                          notificationStore.error(
                                            'Failed to generate install command',
                                          );
                                        } finally {
                                          setLoadingAgentCommand(false);
                                        }
                                      }}
                                      class="absolute top-2 right-2 p-1.5 hover:text-slate-200 bg-surface-hover rounded-md transition-colors disabled:opacity-50"
                                      title="Copy command"
                                    >
                                      <Show
                                        when={loadingAgentCommand()}
                                        fallback={
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
                                        }
                                      >
                                        <svg
                                          class="animate-spin"
                                          width="16"
                                          height="16"
                                          viewBox="0 0 24 24"
                                          fill="none"
                                          stroke="currentColor"
                                          stroke-width="2"
                                        >
                                          <circle
                                            cx="12"
                                            cy="12"
                                            r="10"
                                            stroke-opacity="0.25"
                                          ></circle>
                                          <path
                                            d="M12 2a10 10 0 0 1 10 10"
                                            stroke-linecap="round"
                                          ></path>
                                        </svg>
                                      </Show>
                                    </button>
                                    <Show
                                      when={agentInstallCommand().length > 0}
                                      fallback={
                                        <code class="text-blue-400">
                                          Click the copy button to generate the install command
                                        </code>
                                      }
                                    >
                                      <code class="block text-blue-100 whitespace-pre-wrap break-words">
                                        {agentInstallCommand()}
                                      </code>
                                    </Show>
                                  </div>
                                  <p class="text-[11px] text-muted italic">
                                    The node will appear in Pulse automatically after the agent
                                    starts.
                                  </p>
                                </div>
                              </Show>

                              {/* API Only Tab (formerly Quick Setup) */}
                              <Show when={formData().setupMode === 'auto'}>
                                <div class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 mb-3 dark:border-amber-700 dark:bg-amber-900">
                                  <p class="text-xs text-amber-800 dark:text-amber-200">
                                    <strong>Limited functionality:</strong> API-only mode does not
                                    include temperature monitoring or Pulse Patrol automation. For
                                    full functionality, use the Agent Install tab instead.
                                  </p>
                                </div>
                                <p class="text-blue-800 dark:text-blue-200">
                                  Just copy and run this one command on your Proxmox VE server:
                                </p>

                                {/* One-line command */}
                                <div class="space-y-3">
                                  <div class="relative bg-base rounded-md p-3 font-mono text-xs overflow-x-auto">
                                    <button
                                      type="button"
                                      onClick={async () => {
                                        logger.debug('[Quick Setup] Copy button clicked');
                                        try {
                                          // Check if host is populated
                                          if (!formData().host || formData().host.trim() === '') {
                                            logger.debug('[Quick Setup] No host entered');
                                            notificationStore.error(
                                              'Please enter the Host URL first',
                                            );
                                            return;
                                          }

                                          logger.debug(
                                            '[Quick Setup] Generating setup URL for host',
                                            {
                                              host: formData().host,
                                            },
                                          );
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

                                          logger.debug('[Quick Setup] API response', {
                                            status: response.status,
                                            ok: response.ok,
                                          });
                                          if (response.ok) {
                                            const data = await response.json();
                                            logger.debug('[Quick Setup] Setup data received', data);

                                            setQuickSetupToken(data.setupToken ?? '');
                                            setQuickSetupExpiry(
                                              typeof data.expires === 'number'
                                                ? data.expires
                                                : null,
                                            );

                                            // Backend returns url, command, and expires
                                            // Just copy the command - don't show the modal
                                            if (data.command) {
                                              logger.debug(
                                                '[Quick Setup] Copying command to clipboard',
                                              );
                                              setQuickSetupCommand(data.command);
                                              const copied = await copyToClipboard(data.command);
                                              logger.debug('[Quick Setup] Copy result', copied);
                                              if (copied) {
                                                notificationStore.success(
                                                  'Command copied to clipboard! Paste the setup token shown below when prompted.',
                                                );
                                              } else {
                                                notificationStore.error(
                                                  'Failed to copy to clipboard',
                                                );
                                              }
                                            } else {
                                              logger.debug('[Quick Setup] No command in response');
                                            }
                                          } else {
                                            setQuickSetupToken('');
                                            setQuickSetupExpiry(null);
                                            notificationStore.error('Failed to generate setup URL');
                                          }
                                        } catch (error) {
                                          logger.error('[Quick Setup] Error:', error);
                                          setQuickSetupToken('');
                                          setQuickSetupExpiry(null);
                                          notificationStore.error('Failed to copy command');
                                        }
                                      }}
                                      class="absolute top-2 right-2 p-1.5 text-slate-400 hover:text-slate-200 bg-surface-hover rounded-md transition-colors"
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
                                            ? 'Click the copy button to generate the setup command'
                                            : 'Please enter the Host URL above first'}
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

                                  <div class="bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 rounded-md p-3">
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
                                  <details class="bg-surface-alt rounded-md p-3">
                                    <summary class="cursor-pointer text-sm font-medium text-base-content hover:text-base-content">
                                      Alternative: Download script manually
                                    </summary>
                                    <div class="mt-3 space-y-3">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          try {
                                            const hostValue = formData().host || '';
                                            const encodedHost = encodeURIComponent(hostValue);
                                            const pulseUrl = encodeURIComponent(getPulseBaseUrl());
                                            const scriptUrl = `/api/setup-script?type=pve&host=${encodedHost}&pulse_url=${pulseUrl}&backup_perms=true`;

                                            // Fetch the script using the current session
                                            const response = await apiFetch(scriptUrl);
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

                                            notificationStore.success(
                                              'Script downloaded! Upload it to your server and run: bash pulse-setup.sh',
                                            );
                                          } catch (error) {
                                            logger.error('Failed to download script:', error);
                                            notificationStore.error(
                                              'Failed to download script. Please check your connection.',
                                            );
                                          }
                                        }}
                                        class="w-full px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md transition-colors text-sm font-medium"
                                      >
                                        Download pulse-setup.sh
                                      </button>
                                      <div class="text-xs text-muted">
                                        1. Click to download the script
                                        <br />
                                        2. Upload to your server via SCP/SFTP
                                        <br />
                                        3. Run:{' '}
                                        <code class="bg-surface-alt px-1 rounded">
                                          bash pulse-setup.sh
                                        </code>
                                      </div>
                                    </div>
                                  </details>
                                </div>

                                <div class="bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 rounded-md p-3">
                                  <p class="text-sm font-semibold text-blue-800 dark:text-blue-200 mb-2">
                                    What this does:
                                  </p>
                                  <ul class="text-xs text-blue-700 dark:text-blue-300 space-y-1">
                                    <li class="flex items-start">
                                      <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                                      <span>
                                        Creates monitoring user{' '}
                                        <code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">
                                          pulse-monitor@pam
                                        </code>
                                      </span>
                                    </li>
                                    <li class="flex items-start">
                                      <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                                      <span>Generates secure API token</span>
                                    </li>
                                    <li class="flex items-start">
                                      <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                                      <span>
                                        Sets up monitoring permissions (PVEAuditor + guest agent
                                        access + backup visibility)
                                      </span>
                                    </li>
                                    <li class="flex items-start">
                                      <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                                      <span>Automatically registers node with Pulse</span>
                                    </li>
                                  </ul>
                                  <p class="text-xs text-green-600 dark:text-green-400 mt-2 font-semibold">
                                    Fully automatic - no manual token copying needed!
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
                                    <p class="text-sm font-medium text-base-content mb-1">
                                      1. Create monitoring user:
                                    </p>
                                    <div class="relative bg-surface rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            'pveum user add pulse-monitor@pam --comment "Pulse monitoring service"';
                                          if (await copyToClipboard(cmd)) {
                                            notificationStore.success('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-slate-500 hover:text-base-content transition-colors"
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
                                      <code class="text-base-content">
                                        pveum user add pulse-monitor@pam --comment "Pulse monitoring
                                        service"
                                      </code>
                                    </div>
                                  </div>

                                  {/* Step 2: Generate token */}
                                  <div>
                                    <p class="text-sm font-medium text-base-content mb-1">
                                      2. Generate API token (save the output!):
                                    </p>
                                    <div class="relative bg-surface rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            'pveum user token add pulse-monitor@pam pulse-token --privsep 0';
                                          if (await copyToClipboard(cmd)) {
                                            notificationStore.success('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-slate-500 hover:text-base-content transition-colors"
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
                                      <code class="text-base-content">
                                        pveum user token add pulse-monitor@pam pulse-token --privsep
                                        0
                                      </code>
                                    </div>
                                    <p class="text-amber-600 dark:text-amber-400 text-xs mt-1">
                                      Important: Copy the token value immediately - it won't be
                                      shown again!
                                    </p>
                                  </div>

                                  {/* Step 3: Set permissions */}
                                  <div>
                                    <p class="text-sm font-medium text-base-content mb-1">
                                      3. Set up monitoring permissions:
                                    </p>
                                    <div class="relative bg-surface rounded-md p-2 font-mono text-xs mb-1">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            'pveum aclmod / -user pulse-monitor@pam -role PVEAuditor && if pveum role list 2>/dev/null | grep -q "VM.Monitor" || pveum role add TestMonitor -privs VM.Monitor 2>/dev/null; then pveum role delete TestMonitor 2>/dev/null; pveum role delete PulseMonitor 2>/dev/null; pveum role add PulseMonitor -privs VM.Monitor; pveum aclmod / -user pulse-monitor@pam -role PulseMonitor; fi';
                                          if (await copyToClipboard(cmd)) {
                                            notificationStore.success('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 hover:text-muted transition-colors"
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
                                      <code class="text-base-content whitespace-pre-line">
                                        {
                                          '# Apply monitoring permissions - use built-in PVEAuditor role\npveum aclmod / -user pulse-monitor@pam -role PVEAuditor\n\n# Gather additional privileges for VM metrics\nEXTRA_PRIVS=()\n\n# Sys.Audit (Ceph, cluster status)\nif pveum role list 2>/dev/null | grep -q "Sys.Audit"; then\n  EXTRA_PRIVS+=(\"Sys.Audit\")\nelse\n  if pveum role add PulseTmpSysAudit -privs Sys.Audit 2>/dev/null; then\n    EXTRA_PRIVS+=(\"Sys.Audit\")\n    pveum role delete PulseTmpSysAudit 2>/dev/null\n  fi\nfi\n\n# VM guest agent / monitor privileges\nVM_PRIV=\"\"\nif pveum role list 2>/dev/null | grep -q "VM.Monitor"; then\n  VM_PRIV=\"VM.Monitor\"\nelif pveum role list 2>/dev/null | grep -q "VM.GuestAgent.Audit"; then\n  VM_PRIV=\"VM.GuestAgent.Audit\"\nelse\n  if pveum role add PulseTmpVMMonitor -privs VM.Monitor 2>/dev/null; then\n    VM_PRIV=\"VM.Monitor\"\n    pveum role delete PulseTmpVMMonitor 2>/dev/null\n  elif pveum role add PulseTmpGuestAudit -privs VM.GuestAgent.Audit 2>/dev/null; then\n    VM_PRIV=\"VM.GuestAgent.Audit\"\n    pveum role delete PulseTmpGuestAudit 2>/dev/null\n  fi\nfi\n\nif [ -n \"$VM_PRIV\" ]; then\n  EXTRA_PRIVS+=(\"$VM_PRIV\")\nfi\n\nif [ ${#EXTRA_PRIVS[@]} -gt 0 ]; then\n  PRIV_STRING=\"${EXTRA_PRIVS[*]}\"\n  pveum role delete PulseMonitor 2>/dev/null\n  pveum role add PulseMonitor -privs \"$PRIV_STRING\"\n  pveum aclmod / -user pulse-monitor@pam -role PulseMonitor\nfi'
                                        }
                                      </code>
                                    </div>
                                    <div class="relative bg-surface rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            'pveum aclmod /storage -user pulse-monitor@pam -role PVEDatastoreAdmin';
                                          if (await copyToClipboard(cmd)) {
                                            notificationStore.success('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 hover:text-muted transition-colors"
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
                                      <code class="text-base-content">
                                        pveum aclmod /storage -user pulse-monitor@pam -role
                                        PVEDatastoreAdmin
                                      </code>
                                    </div>
                                    <p class="text-muted text-xs mt-1">
                                      Note: PVEAuditor gives read-only API access. PulseMonitor adds
                                      Sys.Audit plus either VM.Monitor (PVE 8) or
                                      VM.GuestAgent.Audit (PVE 9+) for disk and guest metrics.
                                      PVEDatastoreAdmin on /storage adds backup visibility.
                                    </p>
                                  </div>

                                  {/* Step 4: Use in Pulse */}
                                  <div class="bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-800 rounded-md p-2">
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
                              <div class="flex gap-2 flex-wrap">
                                <button
                                  type="button"
                                  onClick={() => updateField('setupMode', 'agent')}
                                  class={`inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium rounded-md border border-transparent transition-colors ${
                                    formData().setupMode === 'agent'
                                      ? 'bg-surface text-blue-600 dark:text-blue-300 border-border shadow-sm'
                                      : 'text-muted hover:text-blue-600 dark:hover:text-blue-300 hover:bg-surface-hover'
                                  }`}
                                >
                                  Agent Install
                                  <span class="text-[10px] px-1.5 py-0.5 bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">
                                    Recommended
                                  </span>
                                </button>
                                <button
                                  type="button"
                                  onClick={() => {
                                    if (formData().setupMode === 'agent') {
                                      updateField('setupMode', 'auto');
                                    }
                                  }}
                                  class={`inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border border-transparent transition-colors ${
                                    isAdvancedSetupMode()
                                      ? 'bg-surface text-blue-600 dark:text-blue-300 border-border shadow-sm'
                                      : 'text-muted hover:text-blue-600 dark:hover:text-blue-300 hover:bg-surface-hover'
                                  }`}
                                >
                                  Advanced
                                </button>
                              </div>

                              <Show when={isAdvancedSetupMode()}>
                                <div class="mt-1 flex gap-2 flex-wrap pl-0.5">
                                  <button
                                    type="button"
                                    onClick={() => updateField('setupMode', 'auto')}
                                    class={`inline-flex items-center px-2.5 py-1 text-xs font-medium rounded-md border border-transparent transition-colors ${
                                      formData().setupMode === 'auto'
                                        ? 'bg-surface text-blue-600 dark:text-blue-300 border-border shadow-sm'
                                        : 'text-muted hover:text-blue-600 dark:hover:text-blue-300 hover:bg-surface-hover'
                                    }`}
                                  >
                                    API Only
                                  </button>
                                  <button
                                    type="button"
                                    onClick={() => updateField('setupMode', 'manual')}
                                    class={`inline-flex items-center px-2.5 py-1 text-xs font-medium rounded-md border border-transparent transition-colors ${
                                      formData().setupMode === 'manual'
                                        ? 'bg-surface text-blue-600 dark:text-blue-300 border-border shadow-sm'
                                        : 'text-muted hover:text-blue-600 dark:hover:text-blue-300 hover:bg-surface-hover'
                                    }`}
                                  >
                                    Manual Setup
                                  </button>
                                </div>
                              </Show>

                              {/* Agent Install Tab for PBS */}
                              <Show when={formData().setupMode === 'agent'}>
                                <div class="space-y-3">
                                  <p class="text-xs text-muted">
                                    Install the Pulse agent on your Proxmox Backup Server. This is
                                    the recommended method as it provides:
                                  </p>
                                  <ul class="text-xs text-muted list-disc list-inside space-y-1">
                                    <li>
                                      One-command setup (creates API user and token automatically)
                                    </li>
                                    <li>Built-in temperature monitoring (no SSH required)</li>
                                    <li>Pulse features (execute commands via Pulse Assistant)</li>
                                    <li>Automatic reconnection on network issues</li>
                                  </ul>
                                  <p class="text-blue-800 dark:text-blue-200 text-xs mt-3">
                                    Run this command on your PBS node:
                                  </p>
                                  <div class="relative bg-base rounded-md p-3 font-mono text-xs overflow-x-auto">
                                    <button
                                      type="button"
                                      onClick={async () => {
                                        try {
                                          setLoadingAgentCommand(true);
                                          setAgentCommandError(null);
                                          const { apiFetch } = await import('@/utils/apiClient');
                                          const response = await apiFetch(
                                            '/api/agent-install-command',
                                            {
                                              method: 'POST',
                                              headers: { 'Content-Type': 'application/json' },
                                              body: JSON.stringify({ type: 'pbs' }),
                                            },
                                          );
                                          if (!response.ok) {
                                            throw new Error(
                                              `Failed to generate command: ${response.status}`,
                                            );
                                          }
                                          const data = await response.json();
                                          if (data.command) {
                                            setAgentInstallCommand(data.command);
                                            const copied = await copyToClipboard(data.command);
                                            if (copied) {
                                              notificationStore.success(
                                                'Command copied to clipboard',
                                              );
                                            }
                                          }
                                        } catch (error) {
                                          logger.error('[Agent Install] Error:', error);
                                          setAgentCommandError(
                                            error instanceof Error
                                              ? error.message
                                              : 'Failed to generate command',
                                          );
                                          notificationStore.error(
                                            'Failed to generate install command',
                                          );
                                        } finally {
                                          setLoadingAgentCommand(false);
                                        }
                                      }}
                                      class="absolute top-2 right-2 p-1.5 text-slate-400 hover:text-white rounded bg-surface hover:bg-slate-700 transition-colors"
                                      title="Copy to clipboard"
                                      disabled={loadingAgentCommand()}
                                    >
                                      <Show
                                        when={loadingAgentCommand()}
                                        fallback={
                                          <svg
                                            xmlns="http://www.w3.org/2000/svg"
                                            class="h-4 w-4"
                                            fill="none"
                                            viewBox="0 0 24 24"
                                            stroke="currentColor"
                                          >
                                            <path
                                              stroke-linecap="round"
                                              stroke-linejoin="round"
                                              stroke-width="2"
                                              d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                                            />
                                          </svg>
                                        }
                                      >
                                        <svg
                                          class="animate-spin h-4 w-4"
                                          xmlns="http://www.w3.org/2000/svg"
                                          fill="none"
                                          viewBox="0 0 24 24"
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
                                      </Show>
                                    </button>
                                    <code class="text-green-400 whitespace-pre-wrap break-all pr-10">
                                      {agentInstallCommand() ||
                                        'Click the copy button to generate and copy the install command'}
                                    </code>
                                  </div>
                                  <Show when={agentCommandError()}>
                                    <p class="text-xs text-red-500">{agentCommandError()}</p>
                                  </Show>
                                  <p class="text-xs text-muted">
                                    The node will automatically appear in Pulse once the agent
                                    connects.
                                  </p>
                                </div>
                              </Show>

                              {/* Quick Setup Tab for PBS (API Only) */}
                              <Show when={formData().setupMode === 'auto'}>
                                <p class="text-blue-800 dark:text-blue-200">
                                  Just copy and run this one command on your Proxmox Backup Server:
                                </p>

                                {/* One-line command */}
                                <div class="space-y-3">
                                  <div class="relative bg-base rounded-md p-3 font-mono text-xs overflow-x-auto">
                                    <Show when={formData().host && formData().host.trim() !== ''}>
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          try {
                                            // Check if host is populated
                                            if (!formData().host || formData().host.trim() === '') {
                                              notificationStore.error(
                                                'Please enter the Host URL first',
                                              );
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
                                                  notificationStore.success(
                                                    'Command copied to clipboard!',
                                                  );
                                                } else {
                                                  notificationStore.error(
                                                    'Failed to copy to clipboard',
                                                  );
                                                }
                                              }
                                            } else {
                                              notificationStore.error(
                                                'Failed to generate setup URL',
                                              );
                                            }
                                          } catch (error) {
                                            logger.error('Failed to copy command:', error);
                                            notificationStore.error('Failed to copy command');
                                          }
                                        }}
                                        class="absolute top-2 right-2 p-1.5 text-slate-400 hover:text-slate-200 bg-surface-hover rounded-md transition-colors"
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
                                            ? 'Click the copy button to generate the setup command'
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

                                  <div class="bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 rounded-md p-3">
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
                                  <details class="bg-surface-alt rounded-md p-3">
                                    <summary class="cursor-pointer text-sm font-medium text-base-content hover:text-base-content">
                                      Alternative: Download script manually
                                    </summary>
                                    <div class="mt-3 space-y-3">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          try {
                                            const hostValue = formData().host || '';
                                            const encodedHost = encodeURIComponent(hostValue);
                                            const pulseUrl = encodeURIComponent(getPulseBaseUrl());
                                            const scriptUrl = `/api/setup-script?type=pbs&host=${encodedHost}&pulse_url=${pulseUrl}`;

                                            // Fetch the script using the current session
                                            const response = await apiFetch(scriptUrl);
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

                                            notificationStore.success(
                                              'Script downloaded! Upload it to your PBS and run: bash pulse-pbs-setup.sh',
                                            );
                                          } catch (error) {
                                            logger.error('Failed to download script:', error);
                                            notificationStore.error(
                                              'Failed to download script. Please check your connection.',
                                            );
                                          }
                                        }}
                                        class="w-full px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md transition-colors text-sm font-medium"
                                      >
                                        Download pulse-pbs-setup.sh
                                      </button>
                                      <div class="text-xs text-muted">
                                        1. Click to download the script
                                        <br />
                                        2. Upload to your PBS via SCP/SFTP
                                        <br />
                                        3. Run:{' '}
                                        <code class="bg-surface-alt px-1 rounded">
                                          bash pulse-pbs-setup.sh
                                        </code>
                                      </div>
                                    </div>
                                  </details>
                                </div>

                                <div class="bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 rounded-md p-3">
                                  <p class="text-sm font-semibold text-blue-800 dark:text-blue-200 mb-2">
                                    What this does:
                                  </p>
                                  <ul class="text-xs text-blue-700 dark:text-blue-300 space-y-1">
                                    <li class="flex items-start">
                                      <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                                      <span>
                                        Creates monitoring user{' '}
                                        <code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">
                                          pulse-monitor@pbs
                                        </code>
                                      </span>
                                    </li>
                                    <li class="flex items-start">
                                      <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                                      <span>Generates secure API token</span>
                                    </li>
                                    <li class="flex items-start">
                                      <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                                      <span>
                                        Sets up Audit permissions (read-only access to backups +
                                        system stats)
                                      </span>
                                    </li>
                                    <li class="flex items-start">
                                      <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
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
                                    <p class="text-sm font-medium text-base-content mb-1">
                                      1. Create monitoring user:
                                    </p>
                                    <div class="relative bg-surface rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            'proxmox-backup-manager user create pulse-monitor@pbs';
                                          if (await copyToClipboard(cmd)) {
                                            notificationStore.success('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-slate-500 hover:text-base-content transition-colors"
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
                                      <code class="text-base-content">
                                        proxmox-backup-manager user create pulse-monitor@pbs
                                      </code>
                                    </div>
                                  </div>

                                  {/* Step 2: Generate token */}
                                  <div>
                                    <p class="text-sm font-medium text-base-content mb-1">
                                      2. Generate API token (save the output!):
                                    </p>
                                    <div class="relative bg-surface rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            'proxmox-backup-manager user generate-token pulse-monitor@pbs pulse-token';
                                          if (await copyToClipboard(cmd)) {
                                            notificationStore.success('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 text-slate-500 hover:text-base-content transition-colors"
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
                                      <code class="text-base-content">
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
                                    <p class="text-sm font-medium text-base-content mb-1">
                                      3. Set up read-only permissions (includes system stats):
                                    </p>
                                    <div class="relative bg-surface rounded-md p-2 font-mono text-xs mb-1">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            'proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs';
                                          if (await copyToClipboard(cmd)) {
                                            notificationStore.success('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 hover:text-muted transition-colors"
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
                                      <code class="text-base-content">
                                        proxmox-backup-manager acl update / Audit --auth-id
                                        pulse-monitor@pbs
                                      </code>
                                    </div>
                                    <div class="relative bg-surface rounded-md p-2 font-mono text-xs">
                                      <button
                                        type="button"
                                        onClick={async () => {
                                          const cmd =
                                            "proxmox-backup-manager acl update / Audit --auth-id 'pulse-monitor@pbs!pulse-token'";
                                          if (await copyToClipboard(cmd)) {
                                            notificationStore.success('Command copied!');
                                          }
                                        }}
                                        class="absolute top-1 right-1 p-1 hover:text-muted transition-colors"
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
                                      <code class="text-base-content">
                                        proxmox-backup-manager acl update / Audit --auth-id
                                        'pulse-monitor@pbs!pulse-token'
                                      </code>
                                    </div>
                                  </div>

                                  {/* Step 4: Use in Pulse */}
                                  <div class="bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-800 rounded-md p-2">
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
                                  <div class="bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 rounded-md p-2 mt-3">
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
                            <div class="space-y-3 text-xs text-base-content">
                              <p>
                                Generate a dedicated API token in{' '}
                                <strong>Configuration → API Tokens</strong> on your Mail Gateway. We
                                recommend creating a service user such as{' '}
                                <code class="font-mono">pulse-monitor@pmg</code>
                                with <em>Auditor</em> privileges.
                              </p>
                              <ol class="list-decimal ml-4 space-y-1">
                                <li>
                                  Click <em>Add</em> and choose the service user (or create one if
                                  needed).
                                </li>
                                <li>
                                  Enable <em>Privilege Separation</em> and assign the{' '}
                                  <em>Auditor</em> role.
                                </li>
                                <li>
                                  Copy the generated Token ID (e.g.{' '}
                                  <code class="font-mono">pulse-monitor@pmg!pulse-edge</code>) and
                                  the secret value into the fields below.
                                </li>
                              </ol>
                              <p class="text-xs text-muted">
                                Pulse only requires read-only access. Avoid granting administrator
                                permissions to the token.
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
                      titleClass="text-base-content"
                    />
                    <div class="space-y-3">
                      <label class="flex items-center gap-2 text-sm text-base-content">
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
                      titleClass="text-base-content"
                    />
                    <p class="text-sm text-muted">
                      {props.nodeType === 'pmg'
                        ? 'Pulse captures mail flow analytics, rejection causes, and quarantine visibility without additional scripts.'
                        : 'Pulse automatically tracks all supported resources for this node — virtual machines, containers, storage usage, backups, and PBS job activity — so you always get full visibility without extra configuration.'}
                    </p>
                  </div>

                  {/* Physical Disk Monitoring - PVE only */}
                  <Show when={props.nodeType === 'pve'}>
                    <div class="space-y-4">
                      <SectionHeader
                        title="Advanced monitoring"
                        size="sm"
                        class="mb-3"
                        titleClass="text-base-content"
                      />
                      <div class="rounded-md border border-border bg-surface p-3 text-sm shadow-sm">
                        <div class="flex items-start justify-between gap-3">
                          <div>
                            <p class="font-medium text-base-content">
                              Monitor physical disk health (SMART)
                            </p>
                            <p class="mt-1 text-xs text-muted">
                              This will spin up idle HDDs; leave disabled if you rely on drive
                              standby.
                            </p>
                          </div>
                          <TogglePrimitive
                            checked={formData().monitorPhysicalDisks}
                            onChange={(event) =>
                              updateField('monitorPhysicalDisks', event.currentTarget.checked)
                            }
                            ariaLabel={
                              formData().monitorPhysicalDisks
                                ? 'Disable physical disk monitoring'
                                : 'Enable physical disk monitoring'
                            }
                          />
                        </div>
                        <Show when={formData().monitorPhysicalDisks}>
                          <div class="mt-3 flex items-center gap-2 border-t border-border pt-3">
                            <label class="text-xs text-muted">Poll every</label>
                            <select
                              class="rounded border bg-surface px-2 py-1 text-xs text-base-content "
                              value={formData().physicalDiskPollingMinutes}
                              onChange={(e) =>
                                updateField(
                                  'physicalDiskPollingMinutes',
                                  parseInt(e.currentTarget.value, 10),
                                )
                              }
                            >
                              <option value={5}>5 minutes</option>
                              <option value={15}>15 minutes</option>
                              <option value={30}>30 minutes</option>
                              <option value={60}>1 hour</option>
                            </select>
                          </div>
                        </Show>
                      </div>

                      <Show when={showTemperatureMonitoringSection()}>
                        <div class="rounded-md border border-border bg-surface p-3 text-sm shadow-sm">
                          <div class="flex items-start justify-between gap-3">
                            <div>
                              <p class="font-medium text-base-content">Temperature monitoring</p>
                              <p class="mt-1 text-xs text-muted">
                                Uses the Pulse sensors key or proxy to read CPU/NVMe temperatures
                                for this node. Disable if you don't need temperature data or haven't
                                deployed the proxy yet.
                              </p>
                            </div>
                            <TogglePrimitive
                              checked={temperatureMonitoringEnabledValue()}
                              onChange={(event) => {
                                props.onToggleTemperatureMonitoring?.(event.currentTarget.checked);
                              }}
                              disabled={
                                props.savingTemperatureSetting || props.temperatureMonitoringLocked
                              }
                              ariaLabel={
                                temperatureMonitoringEnabledValue()
                                  ? 'Disable temperature monitoring'
                                  : 'Enable temperature monitoring'
                              }
                            />
                          </div>
                          <Show when={!temperatureMonitoringEnabledValue()}>
                            <p class="mt-3 rounded border border-blue-200 bg-blue-50 p-2 text-xs text-blue-700 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200">
                              Pulse will skip SSH temperature polling for this node. Existing
                              dashboard readings will stop refreshing.
                            </p>
                          </Show>
                          <Show when={props.temperatureMonitoringLocked}>
                            <p class="mt-3 rounded border border-amber-200 bg-amber-50 p-2 text-xs text-amber-800 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200">
                              Locked by environment variables. Remove the override
                              (ENABLE_TEMPERATURE_MONITORING) and restart Pulse to manage it in the
                              UI.
                            </p>
                          </Show>
                        </div>
                      </Show>
                    </div>
                  </Show>
                  <Show when={props.nodeType === 'pmg'}>
                    <div class="space-y-3">
                      <SectionHeader
                        title="Data collection"
                        size="sm"
                        class="mb-1"
                        titleClass="text-base-content"
                      />
                      <p class="text-xs text-muted">
                        Control which PMG data sets Pulse ingests. Disable individual collectors if
                        you want to limit API usage.
                      </p>

                      <label class="flex items-start gap-2 text-sm text-base-content">
                        <input
                          type="checkbox"
                          checked={formData().monitorMailStats}
                          onChange={(e) => updateField('monitorMailStats', e.currentTarget.checked)}
                          class={formCheckbox + ' mt-0.5'}
                        />
                        <div>
                          <div>Mail statistics &amp; trends</div>
                          <p class="text-xs text-muted mt-1">
                            Total mail volume, inbound/outbound breakdown, spam and virus counts.
                          </p>
                        </div>
                      </label>

                      <label class="flex items-start gap-2 text-sm text-base-content">
                        <input
                          type="checkbox"
                          checked={formData().monitorQueues}
                          onChange={(e) => updateField('monitorQueues', e.currentTarget.checked)}
                          class={formCheckbox + ' mt-0.5'}
                        />
                        <div>
                          <div>Queue health insights</div>
                          <p class="text-xs text-muted mt-1">
                            Track Postfix queue depth and rejection trends to spot delivery
                            bottlenecks.
                          </p>
                        </div>
                      </label>

                      <label class="flex items-start gap-2 text-sm text-base-content">
                        <input
                          type="checkbox"
                          checked={formData().monitorQuarantine}
                          onChange={(e) =>
                            updateField('monitorQuarantine', e.currentTarget.checked)
                          }
                          class={formCheckbox + ' mt-0.5'}
                        />
                        <div>
                          <div>Quarantine totals</div>
                          <p class="text-xs text-muted mt-1">
                            Mirror PMG quarantine sizes for spam, virus, and attachment buckets.
                          </p>
                        </div>
                      </label>

                      <label class="flex items-start gap-2 text-sm text-base-content">
                        <input
                          type="checkbox"
                          checked={formData().monitorDomainStats}
                          onChange={(e) =>
                            updateField('monitorDomainStats', e.currentTarget.checked)
                          }
                          class={formCheckbox + ' mt-0.5'}
                        />
                        <div>
                          <div>Domain-level statistics</div>
                          <p class="text-xs text-muted mt-1">
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
                    logger.debug('Test result display', {
                      status: result?.status,
                      message: result?.message,
                    });
                    return null;
                  })()}
                  <div
                    class={`mx-6 p-3 rounded-md text-sm ${
                      testResult()?.status === 'success'
                        ? 'bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-800 text-green-800 dark:text-green-200'
                        : testResult()?.status === 'warning'
                          ? 'bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 text-amber-800 dark:text-amber-200'
                          : 'bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 text-red-800 dark:text-red-200'
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
                      <Show when={testResult()?.status === 'warning'}>
                        <svg
                          width="16"
                          height="16"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          stroke-width="2"
                          class="flex-shrink-0 mt-0.5"
                        >
                          <path d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path>
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
                      <div class="flex-1">
                        <p>{testResult()?.message}</p>
                        <Show when={testResult()?.isCluster}>
                          <p class="mt-1 text-xs opacity-80">
                            ✨ Cluster detected! All cluster nodes will be automatically added.
                          </p>
                        </Show>
                        <Show when={testResult()?.warnings && testResult()!.warnings!.length > 0}>
                          <div class="mt-2 space-y-1">
                            <p class="text-xs font-semibold opacity-90">Warnings:</p>
                            <ul class="text-xs space-y-0.5 opacity-80">
                              <For each={testResult()?.warnings}>
                                {(warning) => <li>• {warning}</li>}
                              </For>
                            </ul>
                          </div>
                        </Show>
                      </div>
                    </div>
                  </div>
                </Show>

                {/* Footer */}
                <div class="flex items-center justify-between px-6 py-4 border-t border-border">
                  <button
                    type="button"
                    onClick={handleTestConnection}
                    disabled={isTesting()}
                    class="px-4 py-2 text-sm border border-border text-base-content rounded-md hover:bg-surface-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
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
                        class="px-4 py-2 text-sm border border-border text-base-content rounded-md hover:bg-surface-hover transition-colors flex items-center gap-2"
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
                      class="px-4 py-2 text-sm border border-border text-base-content rounded-md hover:bg-surface-hover transition-colors"
                    >
                      Cancel
                    </button>
                    <button
                      type="submit"
                      class="px-4 py-2 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors"
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
