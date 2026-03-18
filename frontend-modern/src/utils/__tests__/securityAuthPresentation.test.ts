import { describe, expect, it } from 'vitest';
import {
  getSecurityAuthRestartInstruction,
  SECURITY_AUTH_DISABLED_MESSAGE,
  SECURITY_AUTH_DISABLED_PANEL_TITLE,
  SECURITY_AUTH_DISABLED_READ_ONLY_MESSAGE,
  SECURITY_AUTH_RESTART_FOOTER,
  SECURITY_AUTH_RESTART_REQUIRED_MESSAGE,
  SECURITY_AUTH_RESTART_REQUIRED_TITLE,
  SECURITY_AUTH_RESTART_TIP,
  SECURITY_AUTH_SETTINGS_READ_ONLY_MESSAGE,
  SECURITY_AUTH_SETUP_LABEL,
} from '@/utils/securityAuthPresentation';

describe('securityAuthPresentation', () => {
  it('returns canonical auth status copy', () => {
    expect(SECURITY_AUTH_DISABLED_PANEL_TITLE).toBe('Authentication disabled');
    expect(SECURITY_AUTH_SETUP_LABEL).toBe('Setup');
    expect(SECURITY_AUTH_DISABLED_MESSAGE).toContain('Set up password authentication');
    expect(SECURITY_AUTH_DISABLED_READ_ONLY_MESSAGE).toContain('cannot configure it');
    expect(SECURITY_AUTH_SETTINGS_READ_ONLY_MESSAGE).toContain('read-only');
    expect(SECURITY_AUTH_RESTART_REQUIRED_TITLE).toBe('Security Configured - Restart Required');
    expect(SECURITY_AUTH_RESTART_REQUIRED_MESSAGE).toContain('restarted');
    expect(SECURITY_AUTH_RESTART_FOOTER).toContain('log in');
    expect(SECURITY_AUTH_RESTART_TIP).toContain('saved your credentials');
  });

  it('returns deployment-specific restart instructions', () => {
    expect(getSecurityAuthRestartInstruction('docker')).toEqual({
      label: 'Restart your Docker container:',
      command: 'docker restart pulse',
    });
    expect(getSecurityAuthRestartInstruction('proxmoxve')).toEqual({
      label: 'Type update in your ProxmoxVE console',
      secondaryLabel: 'Or restart manually with:',
      command: 'systemctl restart pulse',
    });
    expect(getSecurityAuthRestartInstruction('development')).toEqual({
      label: 'Restart the development server:',
      command: 'sudo systemctl restart pulse-hot-dev',
    });
    expect(getSecurityAuthRestartInstruction()).toEqual({
      label: 'Restart Pulse using your deployment method',
    });
  });
});
