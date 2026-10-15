import { createSignal, onCleanup, type Accessor, type Setter } from 'solid-js';
import { notificationStore } from '@/stores/notifications';
import {
  apiFetchJSON,
  getApiToken as getApiClientToken,
  setApiToken as setApiClientToken,
} from '@/utils/apiClient';
import type { SecurityStatus } from '@/types/config';

export interface BackupTransferStatus {
  kind: 'export' | 'import';
  phase: 'authentication' | 'reading' | 'request' | 'reloading';
}

type Submission = {
  passphrase: string;
  controller: AbortController;
} & ({ kind: 'export' } | { kind: 'import'; file: File });

interface UseBackupTransferFlowParams {
  securityStatus: Accessor<SecurityStatus | null>;
}

// Preserve UI, legacy and CLI archives without treating arbitrary parsed JSON
// (including null, arrays or non-string data) as encrypted configuration.
const encryptedArchive = (content: string): string | null => {
  let parsed: unknown;
  try {
    parsed = JSON.parse(content);
  } catch {
    return content.trim() || null;
  }
  if (typeof parsed === 'string') return parsed.trim() || null;
  if (parsed && typeof parsed === 'object' && !Array.isArray(parsed) && 'data' in parsed) {
    return typeof parsed.data === 'string' ? parsed.data.trim() || null : null;
  }
  return null;
};

export function useBackupTransferFlow({ securityStatus }: UseBackupTransferFlowParams) {
  const [exportPassphrase, setExportPassphrase] = createSignal('');
  const [useCustomPassphrase, setUseCustomPassphrase] = createSignal(false);
  const [importPassphrase, setImportPassphrase] = createSignal('');
  const [importFile, setImportFile] = createSignal<File | null>(null);
  const [showExportDialog, setShowExportDialog] = createSignal(false);
  const [showImportDialog, setShowImportDialog] = createSignal(false);
  const [showApiTokenModal, setShowApiTokenModal] = createSignal(false);
  const [apiTokenInput, setApiTokenInput] = createSignal('');
  const [transferStatus, setTransferStatus] = createSignal<BackupTransferStatus | null>(null);
  const [transferError, setTransferError] = createSignal<string | null>(null);
  let submission: Submission | null = null;
  let disposed = false;
  let reloadTimer: ReturnType<typeof setTimeout> | undefined;

  const active = (current: Submission) => !disposed && submission === current;
  const release = () => {
    submission = null;
    setTransferStatus(null);
  };
  const guard = <T>(get: Accessor<T>, set: Setter<T>): Setter<T> =>
    ((value: T | ((previous: T) => T)) =>
      disposed || submission ? get() : set(value as Parameters<Setter<T>>[0])) as Setter<T>;

  onCleanup(() => {
    disposed = true;
    submission?.controller.abort();
    submission = null;
    if (reloadTimer !== undefined) clearTimeout(reloadTimer);
  });

  const closeExportDialog = () => {
    if (disposed || submission) return;
    setShowExportDialog(false);
    setExportPassphrase('');
    setUseCustomPassphrase(false);
    setTransferError(null);
  };
  const closeImportDialog = () => {
    if (disposed || submission) return;
    setShowImportDialog(false);
    setImportPassphrase('');
    setImportFile(null);
    setTransferError(null);
  };
  const closeApiTokenModal = () => {
    if (disposed || transferStatus()?.phase !== 'authentication') return;
    setShowApiTokenModal(false);
    setApiTokenInput('');
    release();
  };

  const execute = async (current: Submission) => {
    if (!active(current)) return;
    try {
      let data: string | undefined;
      if (current.kind === 'import') {
        setTransferStatus({ kind: current.kind, phase: 'reading' });
        const content = await current.file.text();
        if (!active(current)) return;
        const archive = encryptedArchive(content);
        if (!archive) {
          setTransferError('Invalid backup file. Choose an encrypted configuration backup.');
          return;
        }
        data = archive;
      }

      if (!active(current)) return;
      setTransferStatus({ kind: current.kind, phase: 'request' });
      // Never replay configuration replacement on an auth/CSRF/rate-limit error.
      // Abort stops dispatch if CSRF preparation yields while the view leaves;
      // it cannot roll back a request already sent.
      const result = await apiFetchJSON<{ status: string; data?: string }>(
        `/api/config/${current.kind}`,
        {
          method: 'POST',
          retry: false,
          signal: current.controller.signal,
          body: JSON.stringify({ passphrase: current.passphrase, ...(data ? { data } : {}) }),
        },
      );
      if (!active(current)) return;

      if (current.kind === 'export') {
        const blob = new Blob([JSON.stringify(result, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const anchor = document.createElement('a');
        anchor.href = url;
        anchor.download = `pulse-config-${new Date().toISOString().split('T')[0]}.json`;
        try {
          document.body.appendChild(anchor);
          anchor.click();
        } finally {
          anchor.remove();
          URL.revokeObjectURL(url);
        }
        setShowExportDialog(false);
        setExportPassphrase('');
        setUseCustomPassphrase(false);
        notificationStore.success('Configuration exported successfully');
      } else {
        setImportPassphrase('');
        setImportFile(null);
        setTransferStatus({ kind: 'import', phase: 'reloading' });
        notificationStore.success('Configuration imported successfully. Reloading...');
        reloadTimer = setTimeout(() => {
          if (active(current)) window.location.reload();
        }, 2000);
      }
    } catch {
      if (!active(current)) return;
      // Error text can contain private configuration, paths or tokens. An
      // uncertain write is not an auth prompt or permission to replay it.
      setTransferError(
        transferStatus()?.phase === 'reading'
          ? 'Could not read the backup file. Choose a readable configuration backup.'
          : current.kind === 'import'
            ? 'Could not confirm the import. Settings may already have changed. Check the current configuration before trying again. Verify the backup and passphrase if it was rejected.'
            : 'Could not export the configuration. Check your access and try again.',
      );
    } finally {
      if (active(current) && transferStatus()?.phase !== 'reloading') release();
    }
  };

  const begin = (current: Submission) => {
    submission = current;
    setTransferError(null);
    // Authentication resumes this exact submission, not a later draft.
    if (
      !securityStatus()?.hasAuthentication &&
      securityStatus()?.apiTokenConfigured &&
      !getApiClientToken()
    ) {
      setTransferStatus({ kind: current.kind, phase: 'authentication' });
      setShowApiTokenModal(true);
      return Promise.resolve();
    }
    return execute(current);
  };

  const handleExport = async () => {
    if (disposed || submission || !showExportDialog()) return;
    const passphrase = exportPassphrase();
    if (passphrase.length < 12) {
      setTransferError('Use an encryption passphrase of at least 12 characters.');
      return;
    }
    await begin({ kind: 'export', passphrase, controller: new AbortController() });
  };
  const handleImport = async () => {
    if (disposed || submission || !showImportDialog()) return;
    const passphrase = importPassphrase();
    const file = importFile();
    if (!passphrase || !file) {
      setTransferError('Choose a configuration backup and enter its password.');
      return;
    }
    await begin({ kind: 'import', passphrase, file, controller: new AbortController() });
  };
  const handleApiTokenAuthenticate = () => {
    const current = submission;
    if (disposed || !current || transferStatus()?.phase !== 'authentication') return;
    if (!apiTokenInput().trim()) return;
    setApiClientToken(apiTokenInput());
    setShowApiTokenModal(false);
    setApiTokenInput('');
    void execute(current);
  };

  return {
    exportPassphrase,
    setExportPassphrase: guard(exportPassphrase, setExportPassphrase),
    useCustomPassphrase,
    setUseCustomPassphrase: guard(useCustomPassphrase, setUseCustomPassphrase),
    importPassphrase,
    setImportPassphrase: guard(importPassphrase, setImportPassphrase),
    importFile,
    setImportFile: guard(importFile, setImportFile),
    showExportDialog,
    setShowExportDialog: guard(showExportDialog, setShowExportDialog),
    showImportDialog,
    setShowImportDialog: guard(showImportDialog, setShowImportDialog),
    showApiTokenModal,
    apiTokenInput,
    setApiTokenInput,
    transferStatus,
    transferError,
    handleExport,
    handleImport,
    closeExportDialog,
    closeImportDialog,
    closeApiTokenModal,
    handleApiTokenAuthenticate,
  };
}
