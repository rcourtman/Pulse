import { afterEach, describe, expect, it, vi } from 'vitest';
import { fetchReplicationJobs } from '../ProxmoxReplicationTable';

const job = {
  id: 'pve/100-0',
  instance: 'pve',
  jobId: '100-0',
  enabled: true,
  guestId: 100,
  guestName: 'replicated-vm',
  lastSyncStatus: 'ok',
  failCount: 0,
};
const message =
  'The replication response format is invalid. Job availability is unknown. Missing rows do not mean no jobs exist.';
const respond = (payload: unknown) =>
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify(payload)));

afterEach(() => vi.restoreAllMocks());

describe('replication inventory read boundary', () => {
  it.each([
    null,
    [],
    'BODY_CONTENT_DO_NOT_RENDER',
    {},
    { data: null },
    { data: {} },
    { data: 'BODY_CONTENT_DO_NOT_RENDER' },
    { data: [null] },
    { data: [{}] },
  ])(
    'rejects malformed HTTP 200 inventories rather than reporting no jobs: %j',
    async (payload) => {
      respond(payload);
      await expect(fetchReplicationJobs()).rejects.toThrow(message);
    },
  );

  it.each([
    { id: '' },
    { id: '   ' },
    { id: undefined },
    { instance: undefined },
    { jobId: undefined },
    { enabled: undefined },
    { enabled: 'false' },
    { enabled: 0 },
    { failCount: '0' },
    { failCount: -1 },
    { failCount: 0.5 },
    { guestId: 100.5 },
    { lastSyncStatus: {} },
    { guestName: {} },
    { sourceNode: false },
    { targetNode: [] },
    { schedule: null },
    { lastSyncDurationHuman: 12 },
    { lastSyncDurationSeconds: -1 },
    { lastSyncUnix: '1700000000' },
    { nextSyncUnix: null },
    { lastSyncTime: {} },
    { nextSyncTime: [] },
    { error: { message: 'BODY_CONTENT_DO_NOT_RENDER' } },
    { comment: false },
  ])('rejects the whole read without dropping or coercing a malformed row: %j', async (delta) => {
    respond({ data: [job, { ...job, id: 'pve/101-0', ...delta }] });
    await expect(fetchReplicationJobs()).rejects.toThrow(message);
  });

  it('replaces JSON parser details with a safe inventory error', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('<html>BODY_CONTENT_DO_NOT_RENDER</html>'),
    );
    await expect(fetchReplicationJobs()).rejects.toThrow(message);
  });

  it('preserves an aborted body read instead of inventing a format diagnosis', async () => {
    const controller = new AbortController();
    const response = new Response('');
    const aborted = new DOMException('Read aborted', 'AbortError');
    vi.spyOn(response, 'json').mockImplementation(async () => {
      controller.abort();
      throw aborted;
    });
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(response);
    await expect(fetchReplicationJobs(controller.signal)).rejects.toBe(aborted);
  });

  it('accepts a genuine empty array', async () => {
    respond({ data: [], meta: { total: 0 } });
    await expect(fetchReplicationJobs()).resolves.toEqual([]);
  });

  it('keeps optional observations and unknown provider fields without guessing job state', async () => {
    const rows = [
      { id: 'minimal', instance: '', jobId: '', enabled: false },
      {
        ...job,
        lastSyncStatus: 'provider-state-not-yet-understood',
        lastSyncTime: '2026-10-16T12:00:00Z',
        nextSyncTime: 1_792_152_000_000,
        nextSyncUnix: 1_792_152_000,
        lastSyncDurationSeconds: 0,
        providerAddition: { retained: true },
      },
    ];
    respond({ data: rows, meta: { total: 2 }, futureField: true });
    await expect(fetchReplicationJobs()).resolves.toEqual(rows);
  });
});
