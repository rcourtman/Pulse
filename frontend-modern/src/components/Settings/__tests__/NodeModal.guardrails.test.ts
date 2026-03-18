import { describe, expect, it } from 'vitest';
import nodeModalSource from '../NodeModal.tsx?raw';

describe('NodeModal guardrails', () => {
  it('keeps the manual PVE permission snippet aligned with the canonical setup script', () => {
    expect(nodeModalSource).toContain('const PVE_MANUAL_PERMISSION_COMMAND = `');
    expect(nodeModalSource).toContain('PRIV_STRING="$(IFS=,; echo "\\${EXTRA_PRIVS[*]}")"');
    expect(nodeModalSource).toContain(
      'pveum role modify PulseMonitor -privs "$PRIV_STRING" 2>/dev/null || pveum role add PulseMonitor -privs "$PRIV_STRING" 2>/dev/null',
    );
    expect(nodeModalSource).not.toContain('PRIV_STRING="${EXTRA_PRIVS[*]}"');
    expect(nodeModalSource).not.toContain('pveum role delete PulseMonitor 2>/dev/null');
  });

  it('routes proxmox agent-install commands through the canonical NodesAPI client for both PVE and PBS', () => {
    expect(nodeModalSource).toContain('const copyProxmoxAgentInstallCommand = async (');
    expect(nodeModalSource).toContain("const data = await NodesAPI.getAgentInstallCommand({");
    expect(nodeModalSource).toContain('type,');
    expect(nodeModalSource).toContain('enableProxmox: true,');
    expect(nodeModalSource).toContain("await copyProxmoxAgentInstallCommand(");
    expect(nodeModalSource).toContain("'pve',");
    expect(nodeModalSource).toContain("'pbs',");
    expect(nodeModalSource).toContain("const message = error instanceof Error ? error.message : 'Failed to generate install command';");
    expect(nodeModalSource).toContain("setAgentCommandError(copyFailureMessage);");
    expect(nodeModalSource).toContain("notificationStore.error(copyFailureMessage);");
    expect(nodeModalSource).toContain("notificationStore.error(message);");
    expect(nodeModalSource).not.toContain("apiFetch('/api/agent-install-command'");
  });

  it('routes proxmox setup command and script download transport through the canonical NodesAPI client', () => {
    expect(nodeModalSource).toContain('const [quickSetupBootstrap, setQuickSetupBootstrap] = createSignal<');
    expect(nodeModalSource).toContain('const loadQuickSetupBootstrap = async (');
    expect(nodeModalSource).toContain('await NodesAPI.getProxmoxSetupCommand({');
    expect(nodeModalSource).toContain('const bootstrap = await loadQuickSetupBootstrap(type, backupPerms);');
    expect(nodeModalSource).toContain('const data = await NodesAPI.downloadProxmoxSetupScript(bootstrap);');
    expect(nodeModalSource).toContain('setQuickSetupTokenHint(response.tokenHint);');
    expect(nodeModalSource).toContain('setQuickSetupExpiry(response.expires);');
    expect(nodeModalSource).toContain(
      'setQuickSetupPreviewCommand(response.commandWithoutEnv ?? response.commandWithEnv);',
    );
    expect(nodeModalSource).toContain('copyToClipboard(data.commandWithEnv)');
    expect(nodeModalSource).toContain(
      "Command copied to clipboard! Run it on the server; the one-time setup token is already embedded.",
    );
    expect(nodeModalSource).toContain('Setup token hint:');
    expect(nodeModalSource).toContain("notificationStore.error('Please enter the Endpoint URL first');");
    expect(nodeModalSource).toContain('setQuickSetupBootstrap({ cacheKey, response });');
    expect(nodeModalSource).toContain('if (cached && cached.cacheKey === cacheKey && cached.response.expires > nowUnix)');
    expect(nodeModalSource).toContain('const blob = new Blob([data.content], {');
    expect(nodeModalSource).toContain('type: data.contentType,');
    expect(nodeModalSource).toContain('a.download = data.fileName;');
    expect(nodeModalSource).toContain('const setupScriptRunHint = (fileName: string) => `bash ${fileName}`;');
    expect(nodeModalSource).toContain('setupScriptRunHint(data.fileName)');
    expect(nodeModalSource).not.toContain('Download pulse-setup.sh');
    expect(nodeModalSource).not.toContain('Download pulse-pbs-setup.sh');
    expect(nodeModalSource).not.toContain("apiFetch('/api/setup-script-url'");
    expect(nodeModalSource).not.toContain("const scriptUrl = `/api/setup-script?");
    expect(nodeModalSource).not.toContain('Setup token:</span>');
    expect(nodeModalSource).not.toContain("const [quickSetupToken, setQuickSetupToken] = createSignal('');");
    expect(nodeModalSource).not.toContain('setQuickSetupPreviewCommand(response.commandWithEnv);');
    expect(nodeModalSource).not.toContain('quickSetupCommand()');
    expect(nodeModalSource).not.toContain('Paste the setup token shown below when prompted.');
    expect(nodeModalSource).not.toContain('setQuickSetupToken(');
  });

  it('keeps proxmox setup/install guidance on the monitored-system model instead of reviving agent-era limit copy', () => {
    expect(nodeModalSource).toContain(
      'API-managed PVE/PBS/PMG connections are governed when the connection is',
    );
    expect(nodeModalSource).toContain('monitored-system limit banner.');
    expect(nodeModalSource).not.toContain('Under agents-only counting');
    expect(nodeModalSource).not.toContain('the agent limit');
  });
});
