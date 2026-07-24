import { describe, expect, it, vi, beforeEach } from 'vitest';
import { NodesAPI } from '../nodes';
import type { ProxmoxSetupCommandResponse } from '../nodes';
import type { APITokenRecord } from '@/types/api';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetch: vi.fn(),
  apiFetchJSON: vi.fn(),
}));

describe('NodesAPI — branch coverage (createHostAgentInstallToken)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const validRecord: APITokenRecord = {
    id: 'tok-host-1',
    name: 'host-agent',
    prefix: 'hos_',
    suffix: 'AB12',
    createdAt: '2026-07-24T00:00:00Z',
  };

  it('mints a host agent install token, trims it, and echoes the record when a name is supplied', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      token: ' host-token-xyz ',
      record: validRecord,
    });

    const result = await NodesAPI.createHostAgentInstallToken({
      enableCommands: true,
      name: 'render-east',
    });

    expect(apiFetchJSON).toHaveBeenCalledWith(
      '/api/agent-install-command',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          type: 'host',
          enableCommands: true,
          name: 'render-east',
        }),
      }),
    );
    expect(result).toEqual({ token: 'host-token-xyz', record: validRecord });
  });

  it('omits the name field from the request body entirely when no name is supplied', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      token: 'host-token-xyz',
      record: validRecord,
    });

    await NodesAPI.createHostAgentInstallToken({ enableCommands: false });

    const [, options] = vi.mocked(apiFetchJSON).mock.calls[0]!;
    const body = JSON.parse(options!.body as string);
    expect(body).toEqual({ type: 'host', enableCommands: false });
    expect(body).not.toHaveProperty('name');
  });

  it('rejects with the contract error when the token is blank (short-circuits before checking record)', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      token: '   ',
      record: validRecord,
    });

    await expect(NodesAPI.createHostAgentInstallToken({ enableCommands: true })).rejects.toThrow(
      'Invalid host agent install token response',
    );
  });

  it('rejects with the contract error when the record is absent even though the token is valid', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      token: 'host-token-xyz',
      record: null,
    });

    await expect(NodesAPI.createHostAgentInstallToken({ enableCommands: true })).rejects.toThrow(
      'Invalid host agent install token response',
    );
  });
});

describe('NodesAPI — branch coverage (normalizeProxmoxSetupCommandResponse validation arms)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const validPveResponse = () => ({
    type: 'pve',
    host: 'https://pve.example:8006',
    url: 'https://pulse.example/api/setup-script?type=pve',
    downloadURL: 'https://pulse.example/api/setup-script?type=pve&setup_token=setup-token-123',
    scriptFileName: 'pulse-setup-pve.sh',
    command: 'curl pve ...',
    commandWithEnv: 'curl env pve ...',
    setupToken: 'setup-token-123',
    tokenHint: 'set…123',
    expires: 1_900_000_000,
  });

  it('rejects when host is blank', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({ ...validPveResponse(), host: '' });

    await expect(
      NodesAPI.getProxmoxSetupCommand({ type: 'pve', host: 'pve.example', backupPerms: true }),
    ).rejects.toThrow('Invalid Proxmox setup response host');
  });

  it('rejects when url is blank', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({ ...validPveResponse(), url: '   ' });

    await expect(
      NodesAPI.getProxmoxSetupCommand({ type: 'pve', host: 'pve.example', backupPerms: true }),
    ).rejects.toThrow('Invalid Proxmox setup response URL');
  });

  it('rejects when downloadURL is blank', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({ ...validPveResponse(), downloadURL: '' });

    await expect(
      NodesAPI.getProxmoxSetupCommand({ type: 'pve', host: 'pve.example', backupPerms: true }),
    ).rejects.toThrow('Invalid Proxmox setup response downloadURL');
  });

  it('rejects when command is blank', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({ ...validPveResponse(), command: '' });

    await expect(
      NodesAPI.getProxmoxSetupCommand({ type: 'pve', host: 'pve.example', backupPerms: true }),
    ).rejects.toThrow('Invalid Proxmox setup response command');
  });

  it('rejects when setupToken is blank', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({ ...validPveResponse(), setupToken: '' });

    await expect(
      NodesAPI.getProxmoxSetupCommand({ type: 'pve', host: 'pve.example', backupPerms: true }),
    ).rejects.toThrow('Invalid Proxmox setup response setup token');
  });
});

describe('NodesAPI — branch coverage (downloadProxmoxSetupScript failure arms)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const pveBootstrap = (): ProxmoxSetupCommandResponse => ({
    type: 'pve',
    host: 'https://pve.example:8006',
    url: 'https://pulse.example/base/api/setup-script?type=pve',
    downloadURL: 'https://pulse.example/base/api/setup-script?type=pve&setup_token=setup-token-123',
    scriptFileName: 'pulse-setup-pve.sh',
    command: 'curl pve ...',
    commandWithEnv: 'curl env pve ...',
    commandWithoutEnv: 'curl bare pve ...',
    expires: 1_900_000_000,
    tokenHint: 'set…123',
  });

  it('rejects when the fetch response is not ok (HTTP failure)', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce(new Response('', { status: 502 }));

    await expect(NodesAPI.downloadProxmoxSetupScript(pveBootstrap())).rejects.toThrow(
      'Failed to fetch setup script',
    );
  });

  it('rejects when the script body is empty/whitespace after content-type and filename checks pass', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce(
      new Response('   ', {
        status: 200,
        headers: {
          'Content-Type': 'text/x-shellscript; charset=utf-8',
          'Content-Disposition': 'attachment; filename="pulse-setup-pve.sh"',
        },
      }),
    );

    await expect(NodesAPI.downloadProxmoxSetupScript(pveBootstrap())).rejects.toThrow(
      'Empty Proxmox setup script response',
    );
  });
});
