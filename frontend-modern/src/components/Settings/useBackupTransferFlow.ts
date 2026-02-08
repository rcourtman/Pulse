import { Accessor, createSignal } from 'solid-js';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import {
  apiFetchJSON,
  getApiToken as getApiClientToken,
  setApiToken as setApiClientToken,
} from '@/utils/apiClient';
import type { SecurityStatus } from '@/types/config';

type ApiTokenModalSource = 'export' | 'import' | null;

interface UseBackupTransferFlowParams {
  securityStatus: Accessor<SecurityStatus | null>;
}

export function useBackupTransferFlow({ securityStatus }: UseBackupTransferFlowParams) {
  const [exportPassphrase, setExportPassphrase] = createSignal('');
  const [useCustomPassphrase, setUseCustomPassphrase] = createSignal(false);
  const [importPassphrase, setImportPassphrase] = createSignal('');
  const [importFile, setImportFile] = createSignal<File | null>(null);
  const [showExportDialog, setShowExportDialog] = createSignal(false);
  const [showImportDialog, setShowImportDialog] = createSignal(false);
  const [showApiTokenModal, setShowApiTokenModal] = createSignal(false);
  const [apiTokenInput, setApiTokenInput] = createSignal('');
  const [apiTokenModalSource, setApiTokenModalSource] = createSignal<ApiTokenModalSource>(null);

  const closeExportDialog = () => {
    setShowExportDialog(false);
    setExportPassphrase('');
    setUseCustomPassphrase(false);
  };

  const closeImportDialog = () => {
    setShowImportDialog(false);
    setImportPassphrase('');
    setImportFile(null);
  };

  const closeApiTokenModal = () => {
    setShowApiTokenModal(false);
    setApiTokenInput('');
  };

  const handleExport = async () => {
    if (!exportPassphrase()) {
      const hasAuth = securityStatus()?.hasAuthentication;
      notificationStore.error(
        hasAuth
          ? useCustomPassphrase()
            ? 'Please enter a passphrase'
            : 'Please enter your password'
          : 'Please enter a passphrase',
      );
      return;
    }

    // Backend requires at least 12 characters for encryption security
    if (exportPassphrase().length < 12) {
      const hasAuth = securityStatus()?.hasAuthentication;
      notificationStore.error(
        hasAuth && !useCustomPassphrase()
          ? 'Your password must be at least 12 characters. Please use a custom passphrase instead.'
          : 'Passphrase must be at least 12 characters long',
      );
      return;
    }

    // Only check for API token if user is not authenticated via password
    // If user is logged in with password, session auth is sufficient
    const hasPasswordAuth = securityStatus()?.hasAuthentication;
    if (!hasPasswordAuth && securityStatus()?.apiTokenConfigured && !getApiClientToken()) {
      setApiTokenModalSource('export');
      setShowApiTokenModal(true);
      return;
    }

    try {
      // Get CSRF token from cookie
      const csrfCookie = document.cookie
        .split('; ')
        .find((row) => row.startsWith('pulse_csrf='));
      const csrfToken = csrfCookie
        ? decodeURIComponent(csrfCookie.split('=').slice(1).join('='))
        : undefined;

      const headers: HeadersInit = {
        'Content-Type': 'application/json',
      };

      // Add CSRF token if available
      if (csrfToken) {
        headers['X-CSRF-Token'] = csrfToken;
      }

      // Add API token if configured
      const apiToken = getApiClientToken();
      if (apiToken) {
        headers['X-API-Token'] = apiToken;
      }

      const data = await apiFetchJSON<any>('/api/config/export', {
        method: 'POST',
        body: JSON.stringify({ passphrase: exportPassphrase() }),
      });

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

      notificationStore.success('Configuration exported successfully');
      setShowExportDialog(false);
      setExportPassphrase('');
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : 'Failed to export configuration';
      notificationStore.error(errorMessage);
      logger.error('Export error', error);
    }
  };

  const handleImport = async () => {
    if (!importPassphrase()) {
      notificationStore.error('Please enter the password');
      return;
    }

    if (!importFile()) {
      notificationStore.error('Please select a file to import');
      return;
    }

    // Only check for API token if user is not authenticated via password
    // If user is logged in with password, session auth is sufficient
    const hasPasswordAuth = securityStatus()?.hasAuthentication;
    if (!hasPasswordAuth && securityStatus()?.apiTokenConfigured && !getApiClientToken()) {
      setApiTokenModalSource('import');
      setShowApiTokenModal(true);
      return;
    }

    try {
      const fileContent = await importFile()!.text();

      // Support three formats:
      // 1. UI export: {status: "success", data: "base64string"}
      // 2. Legacy format: {data: "base64string"}
      // 3. CLI export: raw base64 string (no JSON wrapper)
      let encryptedData: string;

      // Try to parse as JSON first
      try {
        const exportData = JSON.parse(fileContent);

        if (typeof exportData === 'string') {
          // Raw base64 string wrapped in JSON (edge case)
          encryptedData = exportData;
        } else if (exportData.data) {
          // Standard format with data field
          encryptedData = exportData.data;
        } else {
          notificationStore.error(
            'Invalid backup file format. Expected encrypted data in "data" field.',
          );
          return;
        }
      } catch (_parseError) {
        // Not JSON - treat entire contents as raw base64 from CLI export
        encryptedData = fileContent.trim();
      }

      await apiFetchJSON('/api/config/import', {
        method: 'POST',
        body: JSON.stringify({
          passphrase: importPassphrase(),
          data: encryptedData,
        }),
      });

      notificationStore.success('Configuration imported successfully. Reloading...');
      setShowImportDialog(false);
      setImportPassphrase('');
      setImportFile(null);

      // Reload page to apply new configuration
      setTimeout(() => window.location.reload(), 2000);
    } catch (error) {
      const errorText = error instanceof Error ? error.message : String(error);

      // Handle specific error cases if possible, though apiFetch usually handles 401/403
      // But for Import, we might want to trigger the token modal if it was a token issue
      // Note: apiFetch throws Error with message.

      if (errorText.includes('API_TOKEN') || errorText.includes('API_TOKENS')) {
        setApiTokenModalSource('import');
        setShowApiTokenModal(true);
        return;
      }

      notificationStore.error(errorText || 'Failed to import configuration');
      logger.error('Import error', error);
    }
  };

  const handleApiTokenAuthenticate = () => {
    if (!apiTokenInput()) {
      notificationStore.error('Please enter the API token');
      return;
    }

    const tokenValue = apiTokenInput()!;
    setApiClientToken(tokenValue);
    const source = apiTokenModalSource();
    setShowApiTokenModal(false);
    setApiTokenInput('');
    setApiTokenModalSource(null);

    // Retry the operation that triggered the modal
    if (source === 'export') {
      void handleExport();
    } else if (source === 'import') {
      void handleImport();
    }
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
    handleExport,
    handleImport,
    closeExportDialog,
    closeImportDialog,
    closeApiTokenModal,
    handleApiTokenAuthenticate,
  };
}
