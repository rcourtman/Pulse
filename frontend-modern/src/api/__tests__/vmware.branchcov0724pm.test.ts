import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { VMwareAPI } from '@/api/vmware';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

const mockedApiFetchJSON = vi.mocked(apiFetchJSON);

describe('VMwareAPI.listConnections — branch coverage (response-shape arms)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('coerces a non-array response to an empty list via the arrayOrUndefined(...) ?? [] fallback', async () => {
    // Backend contract occasionally yields a bare object / null envelope instead of an array.
    // The `?? []` arm must yield [] without throwing.
    mockedApiFetchJSON.mockResolvedValueOnce(null as never);

    await expect(VMwareAPI.listConnections()).resolves.toEqual([]);

    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/vmware/connections');
  });

  it('normalizes a poll.lastError object through the object-guard fall-through of normalizeVMwareConnectionPollError', async () => {
    // Existing specs only ever pass `poll` WITHOUT a lastError, so the `(!error || typeof error !== 'object')`
    // guard always returned early. Supplying a real object exercises the fall-through that builds the trimmed error.
    mockedApiFetchJSON.mockResolvedValueOnce([
      {
        id: 'conn-1',
        host: 'vcsa.lab.local',
        insecureSkipVerify: false,
        enabled: true,
        poll: {
          intervalSeconds: 90,
          lastAttemptAt: ' 2026-04-01T10:00:00Z ',
          lastSuccessAt: ' 2026-04-01T09:59:00Z ',
          consecutiveFailures: 3,
          lastError: {
            at: ' 2026-04-01T10:00:01Z ',
            message: ' connection refused ',
            category: ' transport ',
          },
        },
      },
    ] as never);

    const [connection] = await VMwareAPI.listConnections();

    expect(connection.poll).toEqual({
      intervalSeconds: 90,
      lastAttemptAt: '2026-04-01T10:00:00Z',
      lastSuccessAt: '2026-04-01T09:59:00Z',
      consecutiveFailures: 3,
      lastError: {
        at: '2026-04-01T10:00:01Z',
        message: 'connection refused',
        category: 'transport',
      },
    });
  });

  it('defaults every observed host/vm/datastore/network count to 0 when the raw values are non-finite or absent', async () => {
    // finiteNumberOrUndefined(...) returns undefined for strings/NaN/Infinity/missing values, so each
    // `?? 0` fallback must fire independently for hosts, vms, datastores and networks.
    mockedApiFetchJSON.mockResolvedValueOnce([
      {
        id: 'conn-1',
        host: 'vcsa.lab.local',
        insecureSkipVerify: false,
        enabled: true,
        observed: {
          collectedAt: ' 2026-04-01T00:00:00Z ',
          hosts: 'not-a-number',
          vms: NaN,
          datastores: Infinity,
          // networks intentionally absent
          viRelease: ' 8.0.3 ',
          degraded: true,
          issueCount: 2,
        },
      },
    ] as never);

    const [connection] = await VMwareAPI.listConnections();

    expect(connection.observed).toMatchObject({
      collectedAt: '2026-04-01T00:00:00Z',
      hosts: 0,
      vms: 0,
      datastores: 0,
      networks: 0,
      viRelease: '8.0.3',
      degraded: true,
      issueCount: 2,
    });
  });

  it('falls back to an empty name when the raw connection omits name (optionalTrimmedString(...) ?? "" arm)', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce([
      {
        id: ' conn-2 ',
        host: ' esxi-02.lab.local ',
        insecureSkipVerify: true,
        enabled: false,
        // name intentionally absent
      },
    ] as never);

    const [connection] = await VMwareAPI.listConnections();

    expect(connection).toMatchObject({
      id: 'conn-2',
      name: '',
      host: 'esxi-02.lab.local',
    });
  });
});

describe('VMwareAPI.testSavedConnection — branch coverage (input-absent arm)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('omits the request body entirely when input is undefined (the `: {}` arm of the input ternary)', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce({ success: true } as never);

    await expect(VMwareAPI.testSavedConnection('conn/1')).resolves.toEqual({
      success: true,
      hosts: 0,
      vms: 0,
      datastores: 0,
      networks: 0,
      viRelease: undefined,
      degraded: false,
      issueCount: 0,
      issues: [],
    });

    expect(mockedApiFetchJSON).toHaveBeenCalledWith('/api/vmware/connections/conn%2F1/test', {
      method: 'POST',
    });
    // Belt-and-braces: confirm no `body` key was attached by the spread.
    const [, options] = mockedApiFetchJSON.mock.calls[0]!;
    expect(options).toEqual({ method: 'POST' });
  });
});
