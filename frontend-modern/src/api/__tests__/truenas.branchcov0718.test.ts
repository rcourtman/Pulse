import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { TrueNASAPI, type TrueNASConnectionInput } from '@/api/truenas';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

const fullInput: TrueNASConnectionInput = {
  name: 'tower',
  host: 'truenas.local',
  port: 443,
  apiKey: 'secret',
  username: 'admin',
  password: '********',
  useHttps: true,
  insecureSkipVerify: false,
  fingerprint: 'AA:BB:CC',
  enabled: true,
  pollIntervalSeconds: 60,
  monitorDatasets: true,
  monitorPools: false,
  monitorReplication: true,
};

const emptyPreviewResponse = {
  current_count: 0,
  projected_count: 0,
  additional_count: 0,
  effect: 'no_change',
  current_systems: [],
  projected_systems: [],
};

describe('TrueNASAPI preview* branch coverage', () => {
  const mock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    mock.mockReset();
  });

  describe('previewConnection', () => {
    it('POSTs to /connections/preview and serializes every populated optional field', async () => {
      mock.mockResolvedValueOnce({
        current_count: 1,
        projected_count: 1,
        additional_count: 0,
        effect: 'no_change',
        current_systems: [{ name: 'tower', type: 'truenas', status: 'online', source: 'truenas' }],
        projected_systems: [
          { name: 'tower', type: 'truenas', status: 'online', source: 'truenas' },
        ],
      });

      const result = await TrueNASAPI.previewConnection(fullInput);

      expect(mock).toHaveBeenCalledTimes(1);
      expect(mock).toHaveBeenCalledWith('/api/truenas/connections/preview', {
        method: 'POST',
        body: JSON.stringify({
          name: 'tower',
          host: 'truenas.local',
          port: 443,
          apiKey: 'secret',
          username: 'admin',
          password: '********',
          useHttps: true,
          insecureSkipVerify: false,
          fingerprint: 'AA:BB:CC',
          enabled: true,
          pollIntervalSeconds: 60,
          monitorDatasets: true,
          monitorPools: false,
          monitorReplication: true,
        }),
      });
      // Single projected system is hoisted into projected_system by the normalizer
      expect(result.current_system).toMatchObject({ name: 'tower' });
      expect(result.projected_system).toMatchObject({ name: 'tower' });
      expect(result.effect).toBe('no_change');
      expect(result.projected_count).toBe(1);
    });

    it('emits a host-only body when every optional field is absent', async () => {
      mock.mockResolvedValueOnce(emptyPreviewResponse);

      await TrueNASAPI.previewConnection({ host: 'truenas.local' });

      expect(mock).toHaveBeenCalledWith('/api/truenas/connections/preview', {
        method: 'POST',
        body: JSON.stringify({ host: 'truenas.local' }),
      });
    });

    it('selectively includes only the optional fields that are defined', async () => {
      mock.mockResolvedValueOnce(emptyPreviewResponse);

      await TrueNASAPI.previewConnection({
        host: 'truenas.local',
        port: 8080,
        apiKey: 'k',
        useHttps: false,
        monitorDatasets: true,
      });

      expect(mock).toHaveBeenCalledWith('/api/truenas/connections/preview', {
        method: 'POST',
        body: JSON.stringify({
          host: 'truenas.local',
          port: 8080,
          apiKey: 'k',
          useHttps: false,
          monitorDatasets: true,
        }),
      });
    });

    it('derives current_system from a single current_systems entry when current_system is null', async () => {
      mock.mockResolvedValueOnce({
        current_count: 1,
        projected_count: 0,
        additional_count: -1,
        effect: 'removes_existing',
        current_systems: [
          { name: 'orphan', type: 'truenas', status: 'offline', source: 'truenas' },
        ],
        projected_systems: [],
        current_system: null,
        projected_system: null,
      });

      const result = await TrueNASAPI.previewConnection({ host: 'h' });

      expect(result.current_system).toMatchObject({ name: 'orphan', status: 'offline' });
      expect(result.projected_system).toBeNull();
      expect(result.projected_systems).toEqual([]);
    });

    it('returns null current/projected_system when both system lists are empty', async () => {
      mock.mockResolvedValueOnce(emptyPreviewResponse);

      const result = await TrueNASAPI.previewConnection({ host: 'h' });

      expect(result.current_system).toBeNull();
      expect(result.projected_system).toBeNull();
      expect(result.current_systems).toEqual([]);
      expect(result.projected_systems).toEqual([]);
    });

    it('propagates transport errors from a non-ok preview response', async () => {
      mock.mockRejectedValueOnce(new Error('preview backend unavailable'));

      await expect(TrueNASAPI.previewConnection({ host: 'h' })).rejects.toThrow(
        'preview backend unavailable',
      );
    });
  });

  describe('previewSavedConnection', () => {
    it('encodes the connection id and sends NO body when input is undefined', async () => {
      mock.mockResolvedValueOnce(emptyPreviewResponse);

      await TrueNASAPI.previewSavedConnection('conn/with slash');

      expect(mock).toHaveBeenCalledWith('/api/truenas/connections/conn%2Fwith%20slash/preview', {
        method: 'POST',
      });
    });

    it('includes the serialized body when an input override is provided', async () => {
      mock.mockResolvedValueOnce(emptyPreviewResponse);

      await TrueNASAPI.previewSavedConnection('conn-1', {
        host: 'truenas.local',
        apiKey: 'rotated',
        useHttps: true,
      });

      expect(mock).toHaveBeenCalledWith('/api/truenas/connections/conn-1/preview', {
        method: 'POST',
        body: JSON.stringify({ host: 'truenas.local', apiKey: 'rotated', useHttps: true }),
      });
    });

    it('serializes the full optional-field set when input is fully populated', async () => {
      mock.mockResolvedValueOnce(emptyPreviewResponse);

      await TrueNASAPI.previewSavedConnection('conn-1', fullInput);

      expect(mock).toHaveBeenCalledWith('/api/truenas/connections/conn-1/preview', {
        method: 'POST',
        body: JSON.stringify({
          name: 'tower',
          host: 'truenas.local',
          port: 443,
          apiKey: 'secret',
          username: 'admin',
          password: '********',
          useHttps: true,
          insecureSkipVerify: false,
          fingerprint: 'AA:BB:CC',
          enabled: true,
          pollIntervalSeconds: 60,
          monitorDatasets: true,
          monitorPools: false,
          monitorReplication: true,
        }),
      });
    });

    it('prefers an explicit projected_system over a multi-element projected_systems list', async () => {
      mock.mockResolvedValueOnce({
        current_count: 0,
        projected_count: 2,
        additional_count: 2,
        effect: 'creates_multiple',
        current_systems: [],
        projected_systems: [
          { name: 'p1', type: 'truenas', status: 'online', source: 'truenas' },
          { name: 'p2', type: 'truenas', status: 'warning', source: 'truenas' },
        ],
        projected_system: {
          name: 'primary',
          type: 'truenas',
          status: 'online',
          source: 'truenas',
        },
      });

      const result = await TrueNASAPI.previewSavedConnection('conn-1');

      expect(result.projected_system).toMatchObject({ name: 'primary' });
      expect(result.projected_systems).toHaveLength(2);
    });

    it('falls back to projected_systems[0] only when the list has exactly one element', async () => {
      mock.mockResolvedValueOnce({
        current_count: 0,
        projected_count: 1,
        additional_count: 1,
        effect: 'creates_new',
        current_systems: [],
        projected_systems: [{ name: 'solo', type: 'truenas', status: 'online', source: 'truenas' }],
        projected_system: undefined,
      });

      const result = await TrueNASAPI.previewSavedConnection('conn-1');

      expect(result.projected_system).toMatchObject({ name: 'solo' });
    });

    it('leaves projected_system null when projected_systems is empty and no explicit value is set', async () => {
      mock.mockResolvedValueOnce(emptyPreviewResponse);

      const result = await TrueNASAPI.previewSavedConnection('conn-1');

      expect(result.projected_system).toBeNull();
      expect(result.current_system).toBeNull();
    });

    it('propagates transport errors from a non-ok saved-preview response', async () => {
      mock.mockRejectedValueOnce(new Error('connection not found'));

      await expect(TrueNASAPI.previewSavedConnection('ghost')).rejects.toThrow(
        'connection not found',
      );
    });
  });
});
