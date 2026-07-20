import { describe, expect, it, vi, beforeEach } from 'vitest';
import {
  AgentProfilesAPI,
  INVALID_AGENT_PROFILE_SUGGESTION_MESSAGE,
  INVALID_AGENT_PROFILE_SCHEMA_MESSAGE,
  INVALID_AGENT_PROFILE_VALIDATION_MESSAGE,
  type AgentProfile,
} from '../agentProfiles';
import { apiFetchJSON, apiFetch } from '@/utils/apiClient';
import {
  assertAPIResponseOK,
  assertAPIResponseOKOrAllowedStatus,
  assertAPIResponseOKOrThrowStatus,
  arrayOrEmpty,
  isAPIErrorStatus,
  objectArrayFieldOrEmpty,
  parseRequiredJSON,
  parseRequiredAPIResponse,
  withAPIErrorStatusFallback,
  withAPIErrorStatusNull,
} from '../responseUtils';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
  apiFetch: vi.fn(),
}));

vi.mock('../responseUtils', () => ({
  assertAPIResponseOK: vi.fn(),
  assertAPIResponseOKOrAllowedStatus: vi.fn(),
  assertAPIResponseOKOrThrowStatus: vi.fn(),
  arrayOrEmpty: vi.fn(),
  isAPIErrorStatus: vi.fn(),
  objectArrayFieldOrEmpty: vi.fn(),
  parseRequiredAPIResponse: vi.fn(),
  parseRequiredJSON: vi.fn(),
  withAPIErrorStatusFallback: vi.fn(),
  withAPIErrorStatusNull: vi.fn(),
}));

describe('AgentProfilesAPI branch coverage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(isAPIErrorStatus).mockImplementation((error, expectedStatus) => {
      return (error as { status?: number } | null)?.status === expectedStatus;
    });
    vi.mocked(assertAPIResponseOK).mockResolvedValue(undefined);
    vi.mocked(assertAPIResponseOKOrAllowedStatus).mockResolvedValue(undefined);
    vi.mocked(assertAPIResponseOKOrThrowStatus).mockResolvedValue(undefined);
    vi.mocked(arrayOrEmpty).mockImplementation((value) =>
      Array.isArray(value) ? (value as never[]) : [],
    );
    vi.mocked(objectArrayFieldOrEmpty).mockImplementation((value, field) => {
      if (!value || typeof value !== 'object') {
        return [];
      }
      const fieldValue = (value as Record<string, unknown>)[field];
      return Array.isArray(fieldValue) ? (fieldValue as never[]) : [];
    });
    vi.mocked(parseRequiredJSON).mockImplementation(async (response) => {
      const text = await response.text();
      return JSON.parse(text);
    });
    vi.mocked(withAPIErrorStatusFallback).mockImplementation(
      async (request, fallbackStatus, fallbackValue) => {
        try {
          return await request;
        } catch (error) {
          if ((error as { status?: number } | null)?.status === fallbackStatus) {
            return fallbackValue;
          }
          throw error;
        }
      },
    );
    vi.mocked(withAPIErrorStatusNull).mockImplementation(async (request, nullStatus) => {
      try {
        return await request;
      } catch (error) {
        if ((error as { status?: number } | null)?.status === nullStatus) {
          return null;
        }
        throw error;
      }
    });
    vi.mocked(parseRequiredAPIResponse).mockReset();
  });

  function lastFetchCall(): [string, RequestInit] {
    const calls = vi.mocked(apiFetch).mock.calls as unknown as [string, RequestInit][];
    if (calls.length === 0) {
      throw new Error('apiFetch was not called');
    }
    return calls[calls.length - 1];
  }

  function jsonResponse(body: unknown): Response {
    return new Response(JSON.stringify(body), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  }

  describe('getProfile', () => {
    it('URL-encodes the profile id when it contains path-unsafe characters', async () => {
      const profile: AgentProfile = {
        id: 'a/b c',
        name: 'Profile',
        config: {},
        created_at: '',
        updated_at: '',
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(profile);

      const result = await AgentProfilesAPI.getProfile('a/b c');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/admin/profiles/a%2Fb%20c');
      expect(result).toEqual(profile);
    });

    it('returns null when withAPIErrorStatusNull yields null without throwing', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(null as unknown as AgentProfile);

      const result = await AgentProfilesAPI.getProfile('p1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/admin/profiles/p1');
      expect(result).toBeNull();
    });

    it('preserves optional description and version fields when present', async () => {
      const fullProfile = {
        id: 'p1',
        name: 'Profile',
        description: 'a description',
        config: { a: 1 },
        version: 7,
        created_at: '2024-01-01',
        updated_at: '2024-01-02',
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(fullProfile as unknown as AgentProfile);

      const result = await AgentProfilesAPI.getProfile('p1');

      expect(result).toEqual(fullProfile);
    });
  });

  describe('createProfile', () => {
    it('sends body with description when description is provided', async () => {
      const mockResponse = { ok: true, status: 201 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce({
        id: 'p1',
        name: 'N',
        config: { x: 1 },
        created_at: '',
        updated_at: '',
      });

      await AgentProfilesAPI.createProfile('N', { x: 1 }, 'desc');

      const [path, opts] = lastFetchCall();
      expect(path).toBe('/api/admin/profiles/');
      expect(opts.method).toBe('POST');
      expect(opts.headers).toEqual({ 'Content-Type': 'application/json' });
      expect(JSON.parse(opts.body as string)).toEqual({
        name: 'N',
        description: 'desc',
        config: { x: 1 },
      });
    });

    it('omits description from POST body when description is absent', async () => {
      const mockResponse = { ok: true, status: 201 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce({
        id: 'p1',
        name: 'N',
        config: {},
        created_at: '',
        updated_at: '',
      });

      await AgentProfilesAPI.createProfile('N', {});

      const [, opts] = lastFetchCall();
      const body = JSON.parse(opts.body as string);
      expect(body).toEqual({ name: 'N', config: {} });
      expect(body).not.toHaveProperty('description');
    });
  });

  describe('updateProfile', () => {
    it('sends PUT to URL-encoded id with id+name+description+config in body', async () => {
      const mockResponse = { ok: true, status: 200 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce({
        id: 'a/b',
        name: 'N',
        config: {},
        created_at: '',
        updated_at: '',
      });

      await AgentProfilesAPI.updateProfile('a/b', 'N', {}, 'a description');

      const [path, opts] = lastFetchCall();
      expect(path).toBe('/api/admin/profiles/a%2Fb');
      expect(opts.method).toBe('PUT');
      expect(opts.headers).toEqual({ 'Content-Type': 'application/json' });
      expect(JSON.parse(opts.body as string)).toEqual({
        id: 'a/b',
        name: 'N',
        description: 'a description',
        config: {},
      });
    });

    it('omits description from PUT body when description is absent', async () => {
      const mockResponse = { ok: true, status: 200 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce({
        id: 'p1',
        name: 'N',
        config: {},
        created_at: '',
        updated_at: '',
      });

      await AgentProfilesAPI.updateProfile('p1', 'N', { y: 2 });

      const [, opts] = lastFetchCall();
      const body = JSON.parse(opts.body as string);
      expect(body).toEqual({ id: 'p1', name: 'N', config: { y: 2 } });
      expect(body).not.toHaveProperty('description');
    });
  });

  describe('deleteProfile', () => {
    it('URL-encodes the profile id in the DELETE path', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({
        ok: true,
        status: 204,
      } as unknown as Response);

      await AgentProfilesAPI.deleteProfile('a b/c');

      const [path, opts] = lastFetchCall();
      expect(path).toBe('/api/admin/profiles/a%20b%2Fc');
      expect(opts.method).toBe('DELETE');
    });
  });

  describe('unassignProfile', () => {
    it('URL-encodes the agent id in the DELETE path', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({
        ok: true,
        status: 204,
      } as unknown as Response);

      await AgentProfilesAPI.unassignProfile('agent x/y');

      const [path, opts] = lastFetchCall();
      expect(path).toBe('/api/admin/profiles/assignments/agent%20x%2Fy');
      expect(opts.method).toBe('DELETE');
    });
  });

  describe('suggestProfile', () => {
    it('sends the request object as the JSON body', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true, status: 200 } as unknown as Response);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce({
        name: 'n',
        description: 'd',
        config: {},
        rationale: [],
      });

      await AgentProfilesAPI.suggestProfile({ prompt: 'make it fast' });

      const [path, opts] = lastFetchCall();
      expect(path).toBe('/api/admin/profiles/suggestions');
      expect(opts.method).toBe('POST');
      expect(opts.headers).toEqual({ 'Content-Type': 'application/json' });
      expect(JSON.parse(opts.body as string)).toEqual({ prompt: 'make it fast' });
    });

    it('throws when parsed suggestion is not a record', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true, status: 200 } as unknown as Response);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce('not-a-record' as never);

      await expect(AgentProfilesAPI.suggestProfile({ prompt: 'x' })).rejects.toThrow(
        INVALID_AGENT_PROFILE_SUGGESTION_MESSAGE,
      );
    });

    it('throws when name field is missing from parsed suggestion', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true, status: 200 } as unknown as Response);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce({
        description: 'd',
        config: {},
        rationale: [],
      });

      await expect(AgentProfilesAPI.suggestProfile({ prompt: 'x' })).rejects.toThrow(
        INVALID_AGENT_PROFILE_SUGGESTION_MESSAGE,
      );
    });

    it('throws when description field is missing from parsed suggestion', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true, status: 200 } as unknown as Response);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce({
        name: 'n',
        config: {},
        rationale: [],
      });

      await expect(AgentProfilesAPI.suggestProfile({ prompt: 'x' })).rejects.toThrow(
        INVALID_AGENT_PROFILE_SUGGESTION_MESSAGE,
      );
    });

    it('throws when rationale contains a non-string entry', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true, status: 200 } as unknown as Response);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce({
        name: 'n',
        description: 'd',
        config: {},
        rationale: ['ok', 42],
      });

      await expect(AgentProfilesAPI.suggestProfile({ prompt: 'x' })).rejects.toThrow(
        INVALID_AGENT_PROFILE_SUGGESTION_MESSAGE,
      );
    });

    it('parses multi-entry rationale as a string array on the happy path', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce({ ok: true, status: 200 } as unknown as Response);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce({
        name: 'n',
        description: 'd',
        config: { k: 1 },
        rationale: ['reason 1', 'reason 2'],
      });

      const result = await AgentProfilesAPI.suggestProfile({ prompt: 'x' });

      expect(result).toEqual({
        name: 'n',
        description: 'd',
        config: { k: 1 },
        rationale: ['reason 1', 'reason 2'],
      });
    });
  });

  describe('getConfigSchema', () => {
    it('preserves optional Min/Max/Pattern/Enum fields when present', async () => {
      const schema = [
        {
          Key: 'cpu',
          Type: 'number',
          Description: 'CPU',
          Default: 4,
          Required: true,
          Min: 1,
          Max: 10,
          Pattern: '^\\d+$',
          Enum: ['a', 'b'],
        },
      ];
      vi.mocked(apiFetch).mockResolvedValueOnce(jsonResponse(schema));

      const result = await AgentProfilesAPI.getConfigSchema();

      expect(result).toEqual([
        {
          key: 'cpu',
          type: 'number',
          description: 'CPU',
          defaultValue: 4,
          required: true,
          min: 1,
          max: 10,
          pattern: '^\\d+$',
          enum: ['a', 'b'],
        },
      ]);
    });

    it('returns undefined for optional Min/Max/Pattern/Enum when absent', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        jsonResponse([{ Key: 'k', Type: 'string', Description: 'D', Required: false }]),
      );

      const result = await AgentProfilesAPI.getConfigSchema();

      expect(result).toEqual([
        {
          key: 'k',
          type: 'string',
          description: 'D',
          defaultValue: undefined,
          required: false,
          min: undefined,
          max: undefined,
          pattern: undefined,
          enum: undefined,
        },
      ]);
    });

    it('treats null Enum as absent (undefined)', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        jsonResponse([{ Key: 'k', Type: 'string', Description: 'D', Required: false, Enum: null }]),
      );

      const result = await AgentProfilesAPI.getConfigSchema();

      expect(result[0].enum).toBeUndefined();
    });

    it('throws when an Enum entry is not a string', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        jsonResponse([
          {
            Key: 'k',
            Type: 'string',
            Description: 'D',
            Required: false,
            Enum: ['ok', 5],
          },
        ]),
      );

      await expect(AgentProfilesAPI.getConfigSchema()).rejects.toThrow(
        INVALID_AGENT_PROFILE_SCHEMA_MESSAGE,
      );
    });

    it('throws when a schema definition entry is not a record', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(jsonResponse(['not-a-record']));

      await expect(AgentProfilesAPI.getConfigSchema()).rejects.toThrow(
        INVALID_AGENT_PROFILE_SCHEMA_MESSAGE,
      );
    });
  });

  describe('validateConfig', () => {
    it('sends config object as the JSON body', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(jsonResponse({ Valid: true }));

      await AgentProfilesAPI.validateConfig({ cpu_limit: 4 });

      const [path, opts] = lastFetchCall();
      expect(path).toBe('/api/admin/profiles/validate');
      expect(opts.method).toBe('POST');
      expect(opts.headers).toEqual({ 'Content-Type': 'application/json' });
      expect(JSON.parse(opts.body as string)).toEqual({ cpu_limit: 4 });
    });

    it('returns empty errors array when Errors field is undefined', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(jsonResponse({ Valid: true, Warnings: [] }));

      const result = await AgentProfilesAPI.validateConfig({});

      expect(result).toEqual({ valid: true, errors: [], warnings: [] });
      expect(objectArrayFieldOrEmpty).toHaveBeenCalledWith(
        { Valid: true, Warnings: [] },
        'Warnings',
      );
      expect(objectArrayFieldOrEmpty).not.toHaveBeenCalledWith(expect.anything(), 'Errors');
    });

    it('returns empty warnings array when Warnings field is undefined', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(jsonResponse({ Valid: false, Errors: [] }));

      const result = await AgentProfilesAPI.validateConfig({});

      expect(result).toEqual({ valid: false, errors: [], warnings: [] });
    });

    it('throws when Valid is present but not a boolean', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(jsonResponse({ Valid: 'true' }));

      await expect(AgentProfilesAPI.validateConfig({})).rejects.toThrow(
        INVALID_AGENT_PROFILE_VALIDATION_MESSAGE,
      );
    });

    it('maps Errors and Warnings items to { key, message }', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        jsonResponse({
          Valid: false,
          Errors: [{ Key: 'cpu', Message: 'too high' }],
          Warnings: [{ Key: 'mem', Message: 'low' }],
        }),
      );

      const result = await AgentProfilesAPI.validateConfig({ cpu: 99 });

      expect(result).toEqual({
        valid: false,
        errors: [{ key: 'cpu', message: 'too high' }],
        warnings: [{ key: 'mem', message: 'low' }],
      });
    });
  });
});
