import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type MetricsHistoryParams, type ResourceType } from '@/api/charts';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: vi.fn() }));

const request: MetricsHistoryParams = {
  resourceType: 'agent',
  resourceId: 'pbs-current',
  range: '24h',
};
const point = { timestamp: 1_700_000_000_000, value: 0, min: 0, max: 0 };
const envelope = () => ({
  ...request,
  start: point.timestamp,
  end: point.timestamp + 60_000,
  source: 'store',
});
const all = () => ({ ...envelope(), metrics: { cpu: [point] } });
const single = () => ({ ...envelope(), metric: 'cpu', points: [point] });
const read = async (body: unknown, params = request) => {
  vi.mocked(apiFetchJSON).mockResolvedValueOnce(body);
  return ChartsAPI.getMetricsHistory(params);
};

beforeEach(() => vi.mocked(apiFetchJSON).mockReset());

describe('metrics History response admission', () => {
  it.each([
    ['another resource', { resourceId: 'private-predecessor' }],
    ['another type', { resourceType: 'node' }],
    ['another range', { range: '1h' }],
    ['absent resource', { resourceId: undefined }],
    ['absent type', { resourceType: undefined }],
    ['absent range', { range: undefined }],
    ['non-string resource', { resourceId: 1 }],
    ['unknown source', { source: 'private-source' }],
    ['null source', { source: null }],
    ['absent start', { start: undefined }],
    ['string end', { end: '1700000060000' }],
    ['nonfinite end', { end: Infinity }],
    ['null metrics', { metrics: null }],
    ['array metrics', { metrics: [] }],
    ['missing metrics', { metrics: undefined }],
    ['null series', { metrics: { cpu: null } }],
    ['object series', { metrics: { cpu: point } }],
    ['mixed shapes', { points: [], metric: 'cpu' }],
  ])('rejects %s rather than resolving empty or caching it', async (_label, change) => {
    await expect(read({ ...all(), ...change })).rejects.toThrow(
      'Invalid metrics history response.',
    );
  });

  it.each([null, [], 'private body', 42, {}])('rejects a non-envelope body %#', async (body) => {
    await expect(read(body)).rejects.toThrow('Invalid metrics history response.');
  });

  it.each([
    null,
    1,
    {},
    { ...point, timestamp: '1700000000000' },
    { ...point, timestamp: 9e15 },
    { ...point, value: NaN },
    { ...point, value: null },
    { ...point, min: undefined },
    { ...point, max: Infinity },
  ])('rejects malformed samples in either response shape %#', async (invalid) => {
    await expect(read({ ...all(), metrics: { cpu: [point, invalid] } })).rejects.toThrow(
      'Invalid metrics history response.',
    );
    await expect(
      read({ ...single(), points: [point, invalid] }, { ...request, metric: 'cpu' }),
    ).rejects.toThrow('Invalid metrics history response.');
  });

  it.each([
    { metric: 'memory' },
    { metric: undefined },
    { points: null },
    { points: undefined },
    { points: {} },
    { metrics: {} },
  ])('requires the requested single-metric shape %#', async (change) => {
    await expect(read({ ...single(), ...change }, { ...request, metric: 'cpu' })).rejects.toThrow(
      'Invalid metrics history response.',
    );
  });

  it.each(['store', 'memory', 'live', 'mock_synthetic', undefined])(
    'preserves valid samples and source %s without manufacturing points',
    async (source) => {
      const body = { ...all(), source };
      expect(await read(body)).toBe(body);
      expect(await read({ ...single(), source }, { ...request, metric: 'cpu' })).toEqual({
        ...single(),
        source,
      });
    },
  );

  it('accepts observed empty responses, zero readings and the existing invalid-window fallback', async () => {
    for (const metrics of [{}, { cpu: [] }, { cpu: [point] }]) {
      const body = { ...all(), start: 0, end: 0, metrics };
      expect(await read(body)).toBe(body);
    }
    expect(await read({ ...single(), points: [] }, { ...request, metric: 'cpu' })).toMatchObject({
      points: [],
    });
  });

  it.each(['', '24h'])(
    'accepts the default range echo %s only when range was omitted',
    async (range) => {
      expect(
        await read({ ...all(), range }, { resourceType: 'agent', resourceId: 'pbs-current' }),
      ).toMatchObject({ range });
      await expect(
        read({ ...all(), range: '1h' }, { resourceType: 'agent', resourceId: 'pbs-current' }),
      ).rejects.toThrow();
    },
  );

  it.each([
    'node',
    'agent',
    'vm',
    'system-container',
    'oci-container',
    'app-container',
    'storage',
    'docker-host',
    'disk',
  ] as ResourceType[])(
    'keeps the canonical %s type and server ID trimming',
    async (resourceType) => {
      const body = { ...all(), resourceType };
      expect(await read(body, { ...request, resourceType, resourceId: ' pbs-current ' })).toBe(
        body,
      );
    },
  );

  it.each(['k8s-cluster', 'k8s-node', 'k8s-deployment', 'pod'] as ResourceType[])(
    'accepts the wire k8s token for %s, not an unrelated ID',
    async (resourceType) => {
      const body = { ...all(), resourceType: 'k8s', resourceId: 'k8s:cluster-a:pod:uid' };
      for (const resourceId of ['cluster-a:pod:uid', 'k8s:cluster-a:pod:uid']) {
        expect(await read(body, { ...request, resourceType, resourceId })).toBe(body);
      }
      await expect(
        read(body, { ...request, resourceType, resourceId: 'cluster-b:pod:uid' }),
      ).rejects.toThrow();
    },
  );

  it('does not treat body status claims as HTTP denial or expose wrong-target details', async () => {
    await expect(
      read({ ...all(), resourceId: 'private-resource', status: 403, error: 'private-message' }),
    ).rejects.toMatchObject({
      message: 'Invalid metrics history response.',
    });
  });

  it.each([401, 403, 503])(
    'preserves real transport error %s without reclassifying it',
    async (status) => {
      const error = Object.assign(new Error('transport'), { status });
      vi.mocked(apiFetchJSON).mockRejectedValueOnce(error);
      await expect(ChartsAPI.getMetricsHistory(request)).rejects.toBe(error);
    },
  );

  it('binds validation to the issued selection even if a caller mutates its params', async () => {
    let complete!: (value: unknown) => void;
    vi.mocked(apiFetchJSON).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          complete = resolve;
        }),
    );
    const params = { ...request };
    const promise = ChartsAPI.getMetricsHistory(params);
    params.resourceId = 'successor';
    params.range = '1h';
    complete(all());
    expect(await promise).toMatchObject(request);
  });
});
