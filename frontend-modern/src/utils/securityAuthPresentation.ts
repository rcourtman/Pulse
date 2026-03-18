import type { VersionInfo } from '@/api/updates';

export const SECURITY_AUTH_DISABLED_PANEL_TITLE = 'Authentication disabled';
export const SECURITY_AUTH_SETUP_LABEL = 'Setup';
export const SECURITY_AUTH_DISABLED_MESSAGE =
  'Authentication is currently disabled. Set up password authentication to protect your Pulse instance.';
export const SECURITY_AUTH_DISABLED_READ_ONLY_MESSAGE =
  'This account can view authentication status but cannot configure it.';
export const SECURITY_AUTH_SETTINGS_READ_ONLY_MESSAGE =
  'Authentication settings are read-only for this account.';
export const SECURITY_AUTH_RESTART_REQUIRED_TITLE = 'Security Configured - Restart Required';
export const SECURITY_AUTH_RESTART_REQUIRED_MESSAGE =
  'Security settings have been configured but the service needs to be restarted to activate them.';
export const SECURITY_AUTH_RESTART_FOOTER =
  "After restarting, you'll need to log in with your saved credentials.";
export const SECURITY_AUTH_RESTART_TIP =
  "Tip: Make sure you've saved your credentials before restarting!";

export interface SecurityAuthRestartInstruction {
  label: string;
  command?: string;
  secondaryLabel?: string;
}

export function getSecurityAuthRestartInstruction(
  deploymentType?: VersionInfo['deploymentType'],
): SecurityAuthRestartInstruction {
  switch (deploymentType) {
    case 'proxmoxve':
      return {
        label: 'Type update in your ProxmoxVE console',
        secondaryLabel: 'Or restart manually with:',
        command: 'systemctl restart pulse',
      };
    case 'docker':
      return {
        label: 'Restart your Docker container:',
        command: 'docker restart pulse',
      };
    case 'systemd':
    case 'manual':
      return {
        label: 'Restart the service:',
        command: 'sudo systemctl restart pulse',
      };
    case 'development':
      return {
        label: 'Restart the development server:',
        command: 'sudo systemctl restart pulse-hot-dev',
      };
    default:
      return {
        label: 'Restart Pulse using your deployment method',
      };
  }
}
