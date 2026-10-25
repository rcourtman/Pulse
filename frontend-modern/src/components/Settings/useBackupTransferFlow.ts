import { Accessor, createSignal, onCleanup } from 'solid-js';
import { notificationStore } from '@/stores/notifications';
import {
  apiFetchJSON,
  getOrgID,
  getApiToken as getApiClientToken,
  setApiToken as setApiClientToken,
} from '@/utils/apiClient';
import type { SecurityStatus } from '@/types/config';

interface UseBackupTransferFlowParams {
  securityStatus: Accessor<SecurityStatus | null>;
}

const CHANGED_WORKSPACE =
  'Organisation changed. Close this dialog and reopen it in the intended workspace.';

export function useBackupTransferFlow({ securityStatus }: UseBackupTransferFlowParams) {
  const [exportPassphrase, setExportPassphrase] = createSignal('');
  const [useCustomPassphrase, setUseCustomPassphrase] = createSignal(false);
  const [importPassphrase, setImportPassphrase] = createSignal('');
  const [importFile, setImportFile] = createSignal<File | null>(null);
  const [showExportDialog, showExport] = createSignal(false);
  const [showImportDialog, showImport] = createSignal(false);
  const [showApiTokenModal, setShowApiTokenModal] = createSignal(false);
  const [apiTokenInput, setApiTokenInput] = createSignal('');
  const [transferOrgID, setTransferOrgID] = createSignal('default');
  const [isTransferring, setIsTransferring] = createSignal(false);
  const [transferError, setTransferError] = createSignal('');
  let active = true;
  let controller: AbortController | undefined;
  let reloadTimer: ReturnType<typeof setTimeout> | undefined;

  onCleanup(() => {
    active = false;
    controller?.abort();
    clearTimeout(reloadTimer);
  });

  const currentOrgID = () => getOrgID() || 'default';
  const fail = (message: string) => {
    setTransferError(message);
    notificationStore.error(message);
  };
  const checkWorkspace = () => {
    if (currentOrgID() === transferOrgID()) return true;
    fail(CHANGED_WORKSPACE);
    return false;
  };

  const closeApiTokenModal = () => {
    setShowApiTokenModal(false);
    setApiTokenInput('');
  };
  const closeExportDialog = () => {
    if (isTransferring()) return;
    showExport(false);
    setExportPassphrase('');
    setUseCustomPassphrase(false);
    closeApiTokenModal();
  };
  const closeImportDialog = () => {
    if (isTransferring()) return;
    showImport(false);
    setImportPassphrase('');
    setImportFile(null);
    closeApiTokenModal();
  };
  const openDialog = (kind: 'export' | 'import') => {
    if (isTransferring()) return;
    clearTimeout(reloadTimer);
    closeExportDialog();
    closeImportDialog();
    setTransferError('');
    setTransferOrgID(currentOrgID());
    if (kind === 'export') showExport(true);
    else showImport(true);
  };
  const setShowExportDialog = (open: boolean) =>
    open ? openDialog('export') : closeExportDialog();
  const setShowImportDialog = (open: boolean) =>
    open ? openDialog('import') : closeImportDialog();

  const needsApiToken = () => {
    if (
      !securityStatus()?.hasAuthentication &&
      securityStatus()?.apiTokenConfigured &&
      !getApiClientToken()
    ) {
      setShowApiTokenModal(true);
      return true;
    }
    return false;
  };

  const handleExport = async () => {
    if (!active || !showExportDialog() || isTransferring() || !checkWorkspace()) return;
    setTransferError('');
    const passphrase = exportPassphrase();
    if (!passphrase) {
      fail(
        securityStatus()?.hasAuthentication && !useCustomPassphrase()
          ? 'Please enter your password'
          : 'Please enter a passphrase',
      );
      return;
    }
    if (passphrase.length < 12) {
      fail(
        securityStatus()?.hasAuthentication && !useCustomPassphrase()
          ? 'Your password must be at least 12 characters. Please use a custom passphrase instead.'
          : 'Passphrase must be at least 12 characters long',
      );
      return;
    }
    if (needsApiToken()) return;

    const orgID = transferOrgID();
    controller = new AbortController();
    setIsTransferring(true);
    try {
      const data = await apiFetchJSON('/api/config/export', {
        method: 'POST',
        body: JSON.stringify({ passphrase }),
        expectedOrgID: orgID,
        retry: false,
        signal: controller.signal,
      });
      if (!active || !checkWorkspace()) return;

      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      try {
        a.href = url;
        a.download = `pulse-config-${new Date().toISOString().split('T')[0]}.json`;
        document.body.appendChild(a);
        a.click();
      } finally {
        a.remove();
        URL.revokeObjectURL(url);
      }
      notificationStore.success('Configuration exported successfully');
      showExport(false);
      setExportPassphrase('');
      setUseCustomPassphrase(false);
    } catch (error) {
      if (active) fail(error instanceof Error ? error.message : 'Failed to export configuration');
    } finally {
      if (active) setIsTransferring(false);
    }
  };

  const handleImport = async () => {
    if (!active || !showImportDialog() || isTransferring() || !checkWorkspace()) return;
    setTransferError('');
    const passphrase = importPassphrase();
    const file = importFile();
    if (!passphrase) {
      fail('Please enter the password');
      return;
    }
    if (!file) {
      fail('Please select a file to import');
      return;
    }
    if (needsApiToken()) return;

    // Freeze the confirmed target and inputs before the asynchronous file read.
    const orgID = transferOrgID();
    controller = new AbortController();
    const signal = controller.signal;
    setIsTransferring(true);
    let submitted = false;
    try {
      const fileContent = await file.text();
      if (!active || !checkWorkspace()) return;

      // UI/legacy JSON envelopes, JSON strings and raw CLI base64 remain supported.
      let encryptedData: unknown;
      try {
        const parsed: unknown = JSON.parse(fileContent);
        encryptedData =
          typeof parsed === 'string'
            ? parsed
            : parsed && typeof parsed === 'object' && 'data' in parsed
              ? parsed.data
              : undefined;
      } catch {
        encryptedData = fileContent.trim();
      }
      if (typeof encryptedData !== 'string' || !encryptedData.trim()) {
        fail('Invalid backup file format. Expected encrypted data in "data" field.');
        return;
      }

      submitted = true;
      await apiFetchJSON('/api/config/import', {
        method: 'POST',
        body: JSON.stringify({ passphrase, data: encryptedData }),
        expectedOrgID: orgID,
        retry: false,
        signal,
      });
      if (!active) return;
      if (currentOrgID() !== orgID) {
        fail(
          `Import completed for workspace ${orgID}. The selected workspace changed. No reload was performed.`,
        );
        showImport(false);
        setImportPassphrase('');
        setImportFile(null);
        return;
      }

      notificationStore.success('Configuration imported successfully. Reloading...');
      showImport(false);
      setImportPassphrase('');
      setImportFile(null);
      // Never reload a different workspace or an unmounted Settings view.
      reloadTimer = setTimeout(() => {
        if (active && currentOrgID() === orgID) window.location.reload();
      }, 2000);
    } catch (error) {
      if (!active) return;
      const errorText = error instanceof Error ? error.message : 'Failed to import configuration';
      fail(
        submitted
          ? `${errorText} Import was not confirmed and may have applied. Check the destination before trying again.`
          : errorText,
      );
      if (/api[_ -]?tokens?|x-api-token/i.test(errorText)) setShowApiTokenModal(true);
    } finally {
      if (active) setIsTransferring(false);
    }
  };

  const handleApiTokenAuthenticate = () => {
    if (!active || isTransferring() || !checkWorkspace()) {
      closeApiTokenModal();
      return;
    }
    if (!apiTokenInput().trim()) {
      fail('Please enter the API token');
      return;
    }
    setApiClientToken(apiTokenInput());
    if (!getApiClientToken()) {
      fail('Please enter a valid API token');
      return;
    }
    closeApiTokenModal();
    // Saving a credential is not consent to replay a possibly applied import.
    notificationStore.info('API token saved. Review the workspace and submit the transfer again.');
  };

  return {
    exportPassphrase,
    setExportPassphrase,
    useCustomPassphrase,
    setUseCustomPassphrase,
    importPassphrase,
    setImportPassphrase,
    importFile,
    setImportFile,
    showExportDialog,
    setShowExportDialog,
    showImportDialog,
    setShowImportDialog,
    showApiTokenModal,
    apiTokenInput,
    setApiTokenInput,
    transferOrgID,
    isTransferring,
    transferError,
    handleExport,
    handleImport,
    closeExportDialog,
    closeImportDialog,
    closeApiTokenModal,
    handleApiTokenAuthenticate,
  };
}
