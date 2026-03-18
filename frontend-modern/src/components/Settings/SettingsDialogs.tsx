import { Accessor, Component } from 'solid-js';
import type { UpdateInfo, UpdatePlan, VersionInfo } from '@/api/updates';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';
import { UpdateConfirmationModal } from '@/components/UpdateConfirmationModal';
import { BackupTransferDialogs } from './BackupTransferDialogs';
import { ChangePasswordModal } from './ChangePasswordModal';

interface SettingsDialogsProps {
  showUpdateConfirmation: Accessor<boolean>;
  closeUpdateConfirmation: () => void;
  handleConfirmUpdate: () => void;
  versionInfo: Accessor<VersionInfo | null>;
  updateInfo: Accessor<UpdateInfo | null>;
  updatePlan: Accessor<UpdatePlan | null>;
  isInstallingUpdate: Accessor<boolean>;
  securityStatus: Accessor<SecurityStatusInfo | null>;
  exportPassphrase: Accessor<string>;
  setExportPassphrase: (value: string) => void;
  useCustomPassphrase: Accessor<boolean>;
  setUseCustomPassphrase: (value: boolean) => void;
  importPassphrase: Accessor<string>;
  setImportPassphrase: (value: string) => void;
  importFile: Accessor<File | null>;
  setImportFile: (file: File | null) => void;
  showExportDialog: Accessor<boolean>;
  showImportDialog: Accessor<boolean>;
  showApiTokenModal: Accessor<boolean>;
  apiTokenInput: Accessor<string>;
  setApiTokenInput: (value: string) => void;
  handleExport: () => void;
  handleImport: () => void;
  closeExportDialog: () => void;
  closeImportDialog: () => void;
  closeApiTokenModal: () => void;
  handleApiTokenAuthenticate: () => void;
  showPasswordModal: Accessor<boolean>;
  closePasswordModal: () => void;
}

export const SettingsDialogs: Component<SettingsDialogsProps> = (props) => {
  return (
    <>
      <UpdateConfirmationModal
        isOpen={props.showUpdateConfirmation()}
        onClose={props.closeUpdateConfirmation}
        onConfirm={props.handleConfirmUpdate}
        currentVersion={props.versionInfo()?.version || 'Unknown'}
        latestVersion={props.updateInfo()?.latestVersion || ''}
        plan={
          props.updatePlan() || {
            canAutoUpdate: false,
            requiresRoot: false,
            rollbackSupport: false,
          }
        }
        isApplying={props.isInstallingUpdate()}
        isPrerelease={props.updateInfo()?.isPrerelease}
        isMajorUpgrade={props.updateInfo()?.isMajorUpgrade}
        warning={props.updateInfo()?.warning}
      />

      <BackupTransferDialogs
        securityStatus={props.securityStatus}
        exportPassphrase={props.exportPassphrase}
        setExportPassphrase={props.setExportPassphrase}
        useCustomPassphrase={props.useCustomPassphrase}
        setUseCustomPassphrase={props.setUseCustomPassphrase}
        importPassphrase={props.importPassphrase}
        setImportPassphrase={props.setImportPassphrase}
        importFile={props.importFile}
        setImportFile={props.setImportFile}
        showExportDialog={props.showExportDialog}
        showImportDialog={props.showImportDialog}
        showApiTokenModal={props.showApiTokenModal}
        apiTokenInput={props.apiTokenInput}
        setApiTokenInput={props.setApiTokenInput}
        handleExport={props.handleExport}
        handleImport={props.handleImport}
        closeExportDialog={props.closeExportDialog}
        closeImportDialog={props.closeImportDialog}
        closeApiTokenModal={props.closeApiTokenModal}
        handleApiTokenAuthenticate={props.handleApiTokenAuthenticate}
      />

      <ChangePasswordModal isOpen={props.showPasswordModal()} onClose={props.closePasswordModal} />
    </>
  );
};
