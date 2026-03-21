import { createEffect, createMemo, createSignal } from 'solid-js';
import type { NodeConfig } from '@/types/nodes';

import { notificationStore } from '@/stores/notifications';
import { NodesAPI } from '@/api/nodes';
import type { ProxmoxSetupCommandResponse } from '@/api/nodes';
import { licenseStatus, startProTrial } from '@/stores/license';
import { copyToClipboard } from '@/utils/clipboard';
import { logger } from '@/utils/logger';
import {
  buildNodeModalMonitoringPayload,
  getNodeModalDefaultFormData,
  getNodeModalTestResultPresentation,
  type NodeModalFormData,
} from '@/utils/nodeModalPresentation';
import {
  getProTrialStartedMessage,
  getTrialAlreadyUsedMessage,
  getTrialStartErrorMessage,
  getTrialTryAgainLaterMessage,
} from '@/utils/upgradePresentation';

import { deriveNameFromHost, type NodeModalProps } from './nodeModalModel';

type NodeModalTestResult = {
  status: string;
  message: string;
  isCluster?: boolean;
  warnings?: string[];
};

const PROXMOX_SETUP_HOST_REQUIRED_MESSAGE = 'Proxmox setup host is required';

export const useNodeModalState = (props: NodeModalProps) => {
  const [testResult, setTestResult] = createSignal<NodeModalTestResult | null>(null);
  const [isTesting, setIsTesting] = createSignal(false);
  const [formData, setFormData] = createSignal<NodeModalFormData>(
    getNodeModalDefaultFormData(props.nodeType),
  );
  const [quickSetupBootstrap, setQuickSetupBootstrap] = createSignal<{
    cacheKey: string;
    response: ProxmoxSetupCommandResponse;
  } | null>(null);
  const [quickSetupPreviewCommand, setQuickSetupPreviewCommand] = createSignal('');
  const [quickSetupTokenHint, setQuickSetupTokenHint] = createSignal('');
  const [quickSetupExpiry, setQuickSetupExpiry] = createSignal<number | null>(null);
  const [agentInstallCommand, setAgentInstallCommand] = createSignal('');
  const [loadingAgentCommand, setLoadingAgentCommand] = createSignal(false);
  const [agentCommandError, setAgentCommandError] = createSignal<string | null>(null);

  const isAdvancedSetupMode = () =>
    formData().setupMode === 'auto' || formData().setupMode === 'manual';
  const showTemperatureMonitoringSection = () =>
    typeof props.temperatureMonitoringEnabled === 'boolean';
  const temperatureMonitoringEnabledValue = () => props.temperatureMonitoringEnabled ?? true;
  const isEditingExistingNode = () => Boolean(props.editingNode?.id);

  // API-managed PVE/PBS/PMG connections are governed when the connection is
  // added, not from this install flow, so this modal never renders the
  // monitored-system limit banner.
  const [startingTrial, setStartingTrial] = createSignal(false);
  const hostLimitReached = createMemo(() => false);
  const canStartTrial = createMemo(() => {
    const ent = licenseStatus();
    if (!ent) return false;
    if (ent.subscription_state === 'active' || ent.subscription_state === 'trial') return false;
    return ent.trial_eligible !== false;
  });

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      const result = await startProTrial();
      if (result?.outcome === 'redirect') {
        if (typeof window !== 'undefined') {
          window.location.href = result.actionUrl;
        }
        return;
      }
      notificationStore.success(getProTrialStartedMessage());
    } catch (err) {
      const statusCode = (err as { status?: number } | null)?.status;
      if (statusCode === 409) {
        notificationStore.error(getTrialAlreadyUsedMessage());
      } else if (statusCode === 429) {
        notificationStore.error(getTrialTryAgainLaterMessage());
      } else {
        notificationStore.error(
          getTrialStartErrorMessage(err instanceof Error ? err.message : undefined, {
            branded: true,
          }),
        );
      }
    } finally {
      setStartingTrial(false);
    }
  };

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

  const clearQuickSetupState = () => {
    setQuickSetupBootstrap(null);
    setQuickSetupPreviewCommand('');
    setQuickSetupTokenHint('');
    setQuickSetupExpiry(null);
  };

  const copyCommand = async (command: string, successMessage = 'Command copied!') => {
    if (await copyToClipboard(command)) {
      notificationStore.success(successMessage);
    }
  };

  const copyProxmoxAgentInstallCommand = async (
    type: 'pve' | 'pbs',
    successMessage: string,
  ) => {
    try {
      setLoadingAgentCommand(true);
      setAgentCommandError(null);
      const data = await NodesAPI.getAgentInstallCommand({
        type,
        enableProxmox: true,
      });
      setAgentInstallCommand(data.command);
      const copied = await copyToClipboard(data.command);
      if (copied) {
        notificationStore.success(successMessage);
        return;
      }

      const copyFailureMessage = 'Failed to copy to clipboard';
      setAgentCommandError(copyFailureMessage);
      notificationStore.error(copyFailureMessage);
    } catch (error) {
      logger.error('[Agent Install] Error:', error);
      const message = error instanceof Error ? error.message : 'Failed to generate install command';
      setAgentCommandError(message);
      notificationStore.error(message);
    } finally {
      setLoadingAgentCommand(false);
    }
  };

  const quickSetupCacheKey = (type: 'pve' | 'pbs', backupPerms: boolean) =>
    `${type}:${backupPerms ? 'backup' : 'standard'}:${formData().host?.trim() ?? ''}`;

  const loadQuickSetupBootstrap = async (
    type: 'pve' | 'pbs',
    backupPerms: boolean,
  ): Promise<ProxmoxSetupCommandResponse> => {
    const host = formData().host?.trim() ?? '';
    if (!host) {
      notificationStore.error('Please enter the Endpoint URL first');
      throw new Error(PROXMOX_SETUP_HOST_REQUIRED_MESSAGE);
    }

    const cacheKey = quickSetupCacheKey(type, backupPerms);
    const cached = quickSetupBootstrap();
    const nowUnix = Date.now() / 1000;
    if (cached && cached.cacheKey === cacheKey && cached.response.expires > nowUnix) {
      setQuickSetupPreviewCommand(
        cached.response.commandWithoutEnv ?? cached.response.commandWithEnv,
      );
      setQuickSetupTokenHint(cached.response.tokenHint);
      setQuickSetupExpiry(cached.response.expires);
      return cached.response;
    }

    const response = await NodesAPI.getProxmoxSetupCommand({
      type,
      host,
      backupPerms,
    });
    setQuickSetupBootstrap({ cacheKey, response });
    setQuickSetupPreviewCommand(response.commandWithoutEnv ?? response.commandWithEnv);
    setQuickSetupTokenHint(response.tokenHint);
    setQuickSetupExpiry(response.expires);
    return response;
  };

  const copyQuickSetupCommand = async (
    type: 'pve' | 'pbs',
    backupPerms: boolean,
    successMessage: string,
  ) => {
    logger.debug('[Quick Setup] Copy button clicked');
    try {
      logger.debug('[Quick Setup] Generating setup URL for host', {
        host: formData().host,
      });
      const data = await loadQuickSetupBootstrap(type, backupPerms);
      if (await copyToClipboard(data.commandWithEnv)) {
        notificationStore.success(successMessage);
        return;
      }
      throw new Error('Failed to copy to clipboard');
    } catch (error) {
      logger.error('[Quick Setup] Error:', error);
      clearQuickSetupState();
      if (!(error instanceof Error && error.message === PROXMOX_SETUP_HOST_REQUIRED_MESSAGE)) {
        notificationStore.error('Failed to copy command');
      }
    }
  };

  const setupScriptRunHint = (fileName: string) => `bash ${fileName}`;

  const downloadProxmoxSetupScript = async (type: 'pve' | 'pbs', backupPerms = false) => {
    try {
      const bootstrap = await loadQuickSetupBootstrap(type, backupPerms);
      const data = await NodesAPI.downloadProxmoxSetupScript(bootstrap);

      const blob = new Blob([data.content], {
        type: data.contentType,
      });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = data.fileName;
      document.body.appendChild(anchor);
      anchor.click();
      document.body.removeChild(anchor);
      URL.revokeObjectURL(url);

      notificationStore.success(
        `Script downloaded! Upload it to your server and run: ${setupScriptRunHint(data.fileName)}`,
      );
    } catch (error) {
      logger.error('Failed to download script:', error);
      notificationStore.error('Failed to download script. Please check your connection.');
    }
  };

  const testResultPresentation = createMemo(() =>
    getNodeModalTestResultPresentation(testResult()?.status),
  );

  let previousResetKey: number | undefined;
  let previousNodeType: string | undefined;
  let previousFormSourceSignature: string | null = null;

  createEffect(() => {
    const key = props.resetKey;
    const nodeType = props.nodeType;
    const isOpen = props.isOpen;
    const editingNode = props.editingNode;
    const prefillNode = props.prefillNode;

    if (key !== undefined && key !== previousResetKey) {
      previousResetKey = key;
      setFormData(() => getNodeModalDefaultFormData(props.nodeType));
      clearQuickSetupState();
      setTestResult(null);
      previousFormSourceSignature = null;
      return;
    }

    if (nodeType !== previousNodeType && previousNodeType !== undefined) {
      previousNodeType = nodeType;
      setFormData(() => getNodeModalDefaultFormData(props.nodeType));
      clearQuickSetupState();
      setTestResult(null);
      previousFormSourceSignature = null;
      return;
    }
    previousNodeType = nodeType;

    if (isOpen && !editingNode && !prefillNode) {
      setFormData(() => getNodeModalDefaultFormData(props.nodeType));
      clearQuickSetupState();
      setTestResult(null);
      previousFormSourceSignature = null;
    }
  });

  createEffect(() => {
    const node = props.editingNode ?? props.prefillNode;
    if (!node || node.type !== props.nodeType) {
      previousFormSourceSignature = null;
      return;
    }

    let username = ('user' in node ? node.user : '') || '';
    const tokenName = node.tokenName || '';

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

    const formSource: NodeModalFormData = {
      name: node.name || '',
      host: node.host || '',
      guestURL: ('guestURL' in node ? node.guestURL : '') || '',
      authType: node.type === 'pmg' ? 'password' : node.hasPassword ? 'password' : 'token',
      setupMode: node.source === 'agent' ? 'agent' : 'auto',
      user: username,
      password: '',
      tokenName,
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

  const handleSubmit = (event: Event) => {
    event.preventDefault();
    const data = formData();

    const normalizedName = data.name.trim() || deriveNameFromHost(data.host);
    if (!normalizedName) {
      notificationStore.error('Node name is required');
      return;
    }

    if (normalizedName !== data.name) {
      setFormData((prev) => ({ ...prev, name: normalizedName }));
    }

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
      nodeData.tokenName = data.tokenName;
      if (data.tokenValue) {
        nodeData.tokenValue = data.tokenValue;
      }
    }

    Object.assign(nodeData, buildNodeModalMonitoringPayload(props.nodeType, data));
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
      clearQuickSetupState();
      return;
    }

    setFormData((prev) => ({ ...prev, [field]: value }));
    if (field === 'setupMode') {
      if (value !== 'auto') {
        clearQuickSetupState();
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

    if (isEditingExistingNode()) {
      const hasNewPassword = data.authType === 'password' && data.password;
      const hasNewToken = data.authType === 'token' && data.tokenValue;

      if (!hasNewPassword && !hasNewToken) {
        setIsTesting(true);
        setTestResult(null);

        try {
          const result = await NodesAPI.testExistingNode(props.editingNode!.id);
          setTestResult({
            status: 'success',
            message: result.message || 'Connection successful',
          });
        } catch (error) {
          logger.error('Test existing node error:', error);
          let errorMessage = 'Connection failed';
          if (error instanceof Error) {
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

    if (!data.host) {
      setTestResult({ status: 'error', message: 'Endpoint URL is required' });
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

  return {
    agentCommandError,
    agentInstallCommand,
    canStartTrial,
    copyCommand,
    copyProxmoxAgentInstallCommand,
    copyQuickSetupCommand,
    downloadProxmoxSetupScript,
    formData,
    handleStartTrial,
    handleSubmit,
    handleTestConnection,
    hostLimitReached,
    isAdvancedSetupMode,
    isEditingExistingNode,
    isTesting,
    loadingAgentCommand,
    quickSetupExpiry,
    quickSetupExpiryLabel,
    quickSetupPreviewCommand,
    quickSetupTokenHint,
    showTemperatureMonitoringSection,
    startingTrial,
    temperatureMonitoringEnabledValue,
    testResult,
    testResultPresentation,
    updateField,
  };
};

export type NodeModalState = ReturnType<typeof useNodeModalState>;
