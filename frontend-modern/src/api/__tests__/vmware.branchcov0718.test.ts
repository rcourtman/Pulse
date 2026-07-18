import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { VMwareAPI } from '@/api/vmware';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

const mockedApiFetchJSON = vi.mocked(apiFetchJSON);

const rawEntry = (overrides: Record<string, unknown> = {}): Record<string, unknown> => ({
  name: 'esxi-01',
  type: 'host',
  status: 'online',
  source: 'vmware',
  ...overrides,
});

const basePreviewResponse = (overrides: Record<string, unknown> = {}): Record<string, unknown> => ({
  current_count: 1,
  projected_count: 1,
  additional_count: 0,
  effect: 'attaches_existing',
  current_systems: [rawEntry()],
  projected_systems: [rawEntry({ name: 'esxi-01-projected' })],
  current_system: null,
  projected_system: null,
  ...overrides,
});

const fullInput = {
  name: 'lab-vcenter',
  host: 'vcsa.lab.local',
  port: 443,
  username: 'administrator@vsphere.local',
  password: 'secret',
  insecureSkipVerify: true,
  enabled: true,
  monitorVms: true,
  monitorHosts: false,
  monitorDatastores: true,
};

describe('VMwareAPI.previewConnection — branch coverage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('POSTs the full serialized input (every optional field present) to /api/vmware/connections/preview and normalizes the response', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce(basePreviewResponse() as never);

    const result = await VMwareAPI.previewConnection(fullInput);

    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/vmware/connections/preview', {
      method: 'POST',
      body: JSON.stringify(fullInput),
    });
    expect(result.effect).toBe('attaches_existing');
    expect(result.current_count).toBe(1);
    expect(result.projected_count).toBe(1);
    // explicit current_system/projected_system are null; with single-entry lists
    // the normalizer falls back to the singleton entry.
    expect(result.current_system?.name).toBe('esxi-01');
    expect(result.projected_system?.name).toBe('esxi-01-projected');
    expect(result.current_systems).toHaveLength(1);
    expect(result.projected_systems).toHaveLength(1);
  });

  it('serializes a host-only input to {"host": ...} (every optional field absent branch)', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce(basePreviewResponse() as never);

    await VMwareAPI.previewConnection({ host: 'vcsa.lab.local' });

    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/vmware/connections/preview', {
      method: 'POST',
      body: JSON.stringify({ host: 'vcsa.lab.local' }),
    });
  });

  it('preserves an empty host string verbatim in the serialized body (empty-input branch)', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce(basePreviewResponse() as never);

    await VMwareAPI.previewConnection({ host: '' });

    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/vmware/connections/preview', {
      method: 'POST',
      body: JSON.stringify({ host: '' }),
    });
  });

  it('selects the single current/projected system when explicit singletons are null', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce(
      basePreviewResponse({
        current_systems: [rawEntry({ name: 'only-current' })],
        projected_systems: [rawEntry({ name: 'only-projected' })],
        current_system: null,
        projected_system: null,
      }) as never,
    );

    const result = await VMwareAPI.previewConnection({ host: 'h' });

    expect(result.current_system?.name).toBe('only-current');
    expect(result.projected_system?.name).toBe('only-projected');
  });

  it('nulls current/projected system when both lists are empty and no explicit singleton is provided', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce(
      basePreviewResponse({
        current_count: 0,
        projected_count: 0,
        current_systems: [],
        projected_systems: [],
        current_system: null,
        projected_system: null,
      }) as never,
    );

    const result = await VMwareAPI.previewConnection({ host: 'h' });

    expect(result.current_system).toBeNull();
    expect(result.projected_system).toBeNull();
  });

  it('nulls current/projected system when lists carry multiple entries', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce(
      basePreviewResponse({
        current_systems: [rawEntry({ name: 'a' }), rawEntry({ name: 'b' })],
        projected_systems: [rawEntry({ name: 'c' }), rawEntry({ name: 'd' })],
        current_system: null,
        projected_system: null,
      }) as never,
    );

    const result = await VMwareAPI.previewConnection({ host: 'h' });

    expect(result.current_system).toBeNull();
    expect(result.projected_system).toBeNull();
  });

  it('normalizes explicitly provided current_system and projected_system singletons over the list fallback', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce(
      basePreviewResponse({
        current_systems: [rawEntry({ name: 'list-current' })],
        projected_systems: [rawEntry({ name: 'list-projected' })],
        current_system: rawEntry({ name: 'explicit-current' }),
        projected_system: rawEntry({ name: 'explicit-projected' }),
      }) as never,
    );

    const result = await VMwareAPI.previewConnection({ host: 'h' });

    expect(result.current_system?.name).toBe('explicit-current');
    expect(result.projected_system?.name).toBe('explicit-projected');
  });

  it('propagates transport errors from a non-ok response untouched (error arm)', async () => {
    const transportError = new Error('Request failed with status 502');
    mockedApiFetchJSON.mockRejectedValueOnce(transportError);

    await expect(VMwareAPI.previewConnection({ host: 'h' })).rejects.toBe(transportError);
    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/vmware/connections/preview', {
      method: 'POST',
      body: JSON.stringify({ host: 'h' }),
    });
  });
});

describe('VMwareAPI.previewSavedConnection — branch coverage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('URL-encodes the id and POSTs the serialized input body to <id>/preview (input-present branch)', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce(basePreviewResponse() as never);

    await VMwareAPI.previewSavedConnection('conn/1', fullInput);

    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/vmware/connections/conn%2F1/preview', {
      method: 'POST',
      body: JSON.stringify(fullInput),
    });
  });

  it('omits the body entirely when input is undefined (input-absent branch)', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce(basePreviewResponse() as never);

    await VMwareAPI.previewSavedConnection('conn-1');

    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/vmware/connections/conn-1/preview', {
      method: 'POST',
    });
    // Belt-and-braces: confirm no body key was attached by the spread.
    const [, options] = mockedApiFetchJSON.mock.calls[0]!;
    expect(options).toEqual({ method: 'POST' });
  });

  it('serializes a host-only input body and URL-encodes a slash-containing id', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce(basePreviewResponse() as never);

    await VMwareAPI.previewSavedConnection('site/a', { host: 'vcsa.lab.local' });

    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/vmware/connections/site%2Fa/preview', {
      method: 'POST',
      body: JSON.stringify({ host: 'vcsa.lab.local' }),
    });
  });

  it('normalizes the preview response on the saved-connection route (multi-entry list -> null singleton)', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce(
      basePreviewResponse({
        effect: 'removes_existing',
        current_count: 2,
        projected_count: 1,
        current_systems: [rawEntry({ name: 'cur' }), rawEntry({ name: 'cur2' })],
        projected_systems: [rawEntry({ name: 'proj' })],
        current_system: null,
        projected_system: null,
      }) as never,
    );

    const result = await VMwareAPI.previewSavedConnection('conn-1');

    expect(result.effect).toBe('removes_existing');
    expect(result.current_count).toBe(2);
    expect(result.projected_count).toBe(1);
    expect(result.current_system).toBeNull();
    expect(result.projected_system?.name).toBe('proj');
  });

  it('propagates transport errors from a non-ok response untouched (error arm)', async () => {
    const transportError = new Error('Request failed with status 404');
    mockedApiFetchJSON.mockRejectedValueOnce(transportError);

    await expect(VMwareAPI.previewSavedConnection('conn-1')).rejects.toBe(transportError);
    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/vmware/connections/conn-1/preview', {
      method: 'POST',
    });
  });
});
