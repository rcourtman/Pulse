import { describe, expect, it } from 'vitest';
import nodeModalAuthenticationSectionSource from '../NodeModalAuthenticationSection.tsx?raw';
import nodeModalBasicInfoSectionSource from '../NodeModalBasicInfoSection.tsx?raw';
import nodeModalMonitoringSectionSource from '../NodeModalMonitoringSection.tsx?raw';
import nodeModalSetupGuideSectionSource from '../NodeModalSetupGuideSection.tsx?raw';
import nodeModalStatusFooterSource from '../NodeModalStatusFooter.tsx?raw';
import nodeModalSource from '../NodeModal.tsx?raw';
import nodeModalModelSource from '../nodeModalModel.ts?raw';
import nodeModalStateSource from '../useNodeModalState.ts?raw';

const nodeModalSettingsOwnerSource = [
  nodeModalAuthenticationSectionSource,
  nodeModalBasicInfoSectionSource,
  nodeModalMonitoringSectionSource,
  nodeModalSetupGuideSectionSource,
  nodeModalStatusFooterSource,
  nodeModalSource,
  nodeModalModelSource,
  nodeModalStateSource,
].join('\n');

describe('NodeModal guardrails', () => {
  it('keeps the node modal shell focused on extracted section owners', () => {
    expect(nodeModalSource).toContain('@/components/Settings/NodeModalBasicInfoSection');
    expect(nodeModalSource).toContain('@/components/Settings/NodeModalAuthenticationSection');
    expect(nodeModalSource).toContain('@/components/Settings/NodeModalMonitoringSection');
    expect(nodeModalSource).toContain('@/components/Settings/NodeModalStatusFooter');
    expect(nodeModalSource).not.toContain('title="Basic information"');
    expect(nodeModalSource).not.toContain('title="Authentication"');
    expect(nodeModalSource).not.toContain('title="Monitoring coverage"');
    expect(nodeModalSource).not.toContain('Start your free 14-day trial');
    expect(nodeModalAuthenticationSectionSource).toContain(
      '@/components/Settings/NodeModalSetupGuideSection',
    );
    expect(nodeModalSetupGuideSectionSource).toContain("await state.copyProxmoxAgentInstallCommand(");
    expect(nodeModalMonitoringSectionSource).toContain('title="Monitoring coverage"');
    expect(nodeModalStatusFooterSource).toContain('Start your free 14-day trial');
    expect(nodeModalStateSource).toContain('runStartProTrialAction({');
    expect(nodeModalStateSource).not.toContain('startProTrial()');
  });

  it('keeps the manual PVE permission snippet aligned with the canonical setup script', () => {
    expect(nodeModalSettingsOwnerSource).toContain('export const PVE_MANUAL_PERMISSION_COMMAND = `');
    expect(nodeModalSettingsOwnerSource).toContain(
      'PRIV_STRING="$(IFS=,; echo "\\${EXTRA_PRIVS[*]}")"',
    );
    expect(nodeModalSettingsOwnerSource).toContain(
      'pveum role modify PulseMonitor -privs "$PRIV_STRING" 2>/dev/null || pveum role add PulseMonitor -privs "$PRIV_STRING" 2>/dev/null',
    );
    expect(nodeModalSettingsOwnerSource).not.toContain('PRIV_STRING="${EXTRA_PRIVS[*]}"');
    expect(nodeModalSettingsOwnerSource).not.toContain('pveum role delete PulseMonitor 2>/dev/null');
  });

  it('routes proxmox agent-install commands through the canonical NodesAPI client for both PVE and PBS', () => {
    expect(nodeModalSource).toContain('useNodeModalState');
    expect(nodeModalStateSource).toContain("from '@/stores/licenseCommercial';");
    expect(nodeModalStateSource).not.toContain("from '@/stores/license';\n");
    expect(nodeModalStateSource).toContain('void loadLicenseStatus();');
    expect(nodeModalStateSource).toContain('const ent = licenseStatus();');
    expect(nodeModalStateSource).toContain('const copyProxmoxAgentInstallCommand = async (');
    expect(nodeModalStateSource).toContain("const data = await NodesAPI.getAgentInstallCommand({");
    expect(nodeModalStateSource).toContain('type,');
    expect(nodeModalStateSource).toContain('enableProxmox: true,');
    expect(nodeModalSettingsOwnerSource).toContain("await state.copyProxmoxAgentInstallCommand(");
    expect(nodeModalSettingsOwnerSource).toContain("'pve',");
    expect(nodeModalSettingsOwnerSource).toContain("'pbs',");
    expect(nodeModalStateSource).toContain(
      "const message = error instanceof Error ? error.message : 'Failed to generate install command';",
    );
    expect(nodeModalStateSource).toContain("setAgentCommandError(copyFailureMessage);");
    expect(nodeModalStateSource).toContain("notificationStore.error(copyFailureMessage);");
    expect(nodeModalStateSource).toContain("notificationStore.error(message);");
    expect(nodeModalSettingsOwnerSource).not.toContain("apiFetch('/api/agent-install-command'");
  });

  it('routes proxmox setup command and script download transport through the canonical NodesAPI client', () => {
    expect(nodeModalStateSource).toContain(
      'const [quickSetupBootstrap, setQuickSetupBootstrap] = createSignal<',
    );
    expect(nodeModalStateSource).toContain('const loadQuickSetupBootstrap = async (');
    expect(nodeModalStateSource).toContain('await NodesAPI.getProxmoxSetupCommand({');
    expect(nodeModalStateSource).toContain(
      'const bootstrap = await loadQuickSetupBootstrap(type, backupPerms);',
    );
    expect(nodeModalStateSource).toContain(
      'const data = await NodesAPI.downloadProxmoxSetupScript(bootstrap);',
    );
    expect(nodeModalStateSource).toContain('setQuickSetupTokenHint(response.tokenHint);');
    expect(nodeModalStateSource).toContain('setQuickSetupExpiry(response.expires);');
    expect(nodeModalStateSource).toContain(
      'setQuickSetupPreviewCommand(response.commandWithoutEnv ?? response.commandWithEnv);',
    );
    expect(nodeModalStateSource).toContain('copyToClipboard(data.commandWithEnv)');
    expect(nodeModalSettingsOwnerSource).toContain(
      "Command copied to clipboard! Run it on the server; the one-time setup token is already embedded.",
    );
    expect(nodeModalSettingsOwnerSource).toContain('Setup token hint:');
    expect(nodeModalStateSource).toContain(
      "notificationStore.error('Please enter the Endpoint URL first');",
    );
    expect(nodeModalStateSource).toContain('setQuickSetupBootstrap({ cacheKey, response });');
    expect(nodeModalStateSource).toContain(
      'if (cached && cached.cacheKey === cacheKey && cached.response.expires > nowUnix)',
    );
    expect(nodeModalStateSource).toContain('const blob = new Blob([data.content], {');
    expect(nodeModalStateSource).toContain('type: data.contentType,');
    expect(nodeModalStateSource).toContain('anchor.download = data.fileName;');
    expect(nodeModalStateSource).toContain(
      'const setupScriptRunHint = (fileName: string) => `bash ${fileName}`;',
    );
    expect(nodeModalStateSource).toContain('setupScriptRunHint(data.fileName)');
    expect(nodeModalSettingsOwnerSource).not.toContain('Download pulse-setup.sh');
    expect(nodeModalSettingsOwnerSource).not.toContain('Download pulse-pbs-setup.sh');
    expect(nodeModalSettingsOwnerSource).not.toContain("apiFetch('/api/setup-script-url'");
    expect(nodeModalSettingsOwnerSource).not.toContain("const scriptUrl = `/api/setup-script?");
    expect(nodeModalSettingsOwnerSource).not.toContain('Setup token:</span>');
    expect(nodeModalSettingsOwnerSource).not.toContain(
      "const [quickSetupToken, setQuickSetupToken] = createSignal('');",
    );
    expect(nodeModalSettingsOwnerSource).not.toContain(
      'setQuickSetupPreviewCommand(response.commandWithEnv);',
    );
    expect(nodeModalSettingsOwnerSource).not.toContain('quickSetupCommand()');
    expect(nodeModalSettingsOwnerSource).not.toContain(
      'Paste the setup token shown below when prompted.',
    );
    expect(nodeModalSettingsOwnerSource).not.toContain('setQuickSetupToken(');
  });

  it('keeps proxmox setup/install guidance on the monitored-system model instead of reviving agent-era limit copy', () => {
    expect(nodeModalStateSource).toContain(
      'API-managed PVE/PBS/PMG connections are governed when the connection is',
    );
    expect(nodeModalStateSource).toContain('const hostLimitReached = createMemo(() => false);');
    expect(nodeModalStateSource).toContain("if (ent.subscription_state === 'active' || ent.subscription_state === 'trial')");
    expect(nodeModalStateSource).toContain('return ent.trial_eligible !== false;');
    expect(nodeModalStateSource).toContain('monitored-system limit banner.');
    expect(nodeModalSettingsOwnerSource).not.toContain('Under agents-only counting');
    expect(nodeModalSettingsOwnerSource).not.toContain('the agent limit');
  });
});
