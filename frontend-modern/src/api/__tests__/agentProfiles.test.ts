import { describe, expect, it, vi, beforeEach } from 'vitest';
import {
  AgentProfilesAPI,
  INVALID_AGENT_PROFILE_RESPONSE_MESSAGE,
  INVALID_AGENT_PROFILE_ASSIGNMENT_LIST_MESSAGE,
  INVALID_AGENT_PROFILE_LIST_MESSAGE,
  INVALID_AGENT_PROFILE_SCHEMA_MESSAGE,
  INVALID_AGENT_PROFILE_SUGGESTION_MESSAGE,
  INVALID_AGENT_PROFILE_VALIDATION_MESSAGE,
  MISSING_AGENT_PROFILE_ASSIGNMENT_MESSAGE,
  type AgentProfile,
  type AgentProfileAssignment,
} from '../agentProfiles';
import { apiFetchJSON, apiFetch } from '@/utils/apiClient';
import {
  assertAPIResponseOK,
  assertAPIResponseOKOrAllowedStatus,
  assertAPIResponseOKOrThrowStatus,
  isAPIErrorStatus,
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
  withAPIErrorStatusFallback: vi.fn(),
  withAPIErrorStatusNull: vi.fn(),
}));

describe('AgentProfilesAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(isAPIErrorStatus).mockImplementation((error, expectedStatus) => {
      return (error as { status?: number } | null)?.status === expectedStatus;
    });
    vi.mocked(assertAPIResponseOK).mockResolvedValue(undefined);
    vi.mocked(assertAPIResponseOKOrAllowedStatus).mockResolvedValue(undefined);
    vi.mocked(assertAPIResponseOKOrThrowStatus).mockResolvedValue(undefined);
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

  describe('listProfiles', () => {
    it('fetches all profiles', async () => {
      const mockProfiles: AgentProfile[] = [
        { id: 'p1', name: 'Profile 1', config: {}, created_at: '', updated_at: '' },
      ];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockProfiles);

      const result = await AgentProfilesAPI.listProfiles();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/admin/profiles/');
      expect(withAPIErrorStatusFallback).toHaveBeenCalledWith(expect.any(Promise), 402, []);
      expect(result).toEqual(mockProfiles);
    });

    it('returns empty array on 402 error', async () => {
      vi.mocked(apiFetchJSON).mockRejectedValueOnce(Object.assign(new Error('Payment Required'), { status: 402 }));

      const result = await AgentProfilesAPI.listProfiles();

      expect(result).toEqual([]);
    });

    it('fails closed when the profiles response is null', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(null);

      await expect(AgentProfilesAPI.listProfiles()).rejects.toThrow(
        INVALID_AGENT_PROFILE_LIST_MESSAGE,
      );
    });

    it('fails closed when the profiles response is malformed', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ profiles: [] } as any);

      await expect(AgentProfilesAPI.listProfiles()).rejects.toThrow(
        INVALID_AGENT_PROFILE_LIST_MESSAGE,
      );
    });

    it('fails closed when a profile entry is malformed', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce([
        { id: 'p1', name: 'Profile 1', config: null, created_at: '', updated_at: '' },
      ] as any);

      await expect(AgentProfilesAPI.listProfiles()).rejects.toThrow(
        INVALID_AGENT_PROFILE_LIST_MESSAGE,
      );
    });
  });

  describe('getProfile', () => {
    it('fetches a single profile', async () => {
      const mockProfile: AgentProfile = {
        id: 'p1',
        name: 'Profile 1',
        config: {},
        created_at: '',
        updated_at: '',
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockProfile);

      const result = await AgentProfilesAPI.getProfile('p1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/admin/profiles/p1');
      expect(withAPIErrorStatusNull).toHaveBeenCalledWith(expect.any(Promise), 404);
      expect(result).toEqual(mockProfile);
    });

    it('returns null on 404', async () => {
      vi.mocked(apiFetchJSON).mockRejectedValueOnce(Object.assign(new Error('Not Found'), { status: 404 }));

      const result = await AgentProfilesAPI.getProfile('p1');

      expect(result).toBeNull();
    });

    it('fails closed when the profile response is malformed', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ id: 'p1', config: {} } as any);

      await expect(AgentProfilesAPI.getProfile('p1')).rejects.toThrow(
        INVALID_AGENT_PROFILE_RESPONSE_MESSAGE,
      );
    });
  });

  describe('createProfile', () => {
    it('creates a new profile', async () => {
      const mockResponse = {
        ok: true,
        status: 200,
        text: () =>
          Promise.resolve(
            JSON.stringify({
              id: 'p1',
              name: 'New Profile',
              config: { key: 'value' },
              created_at: '',
              updated_at: '',
            }),
          ),
      } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce({
        id: 'p1',
        name: 'New Profile',
        config: { key: 'value' },
        created_at: '',
        updated_at: '',
      } as AgentProfile);

      await AgentProfilesAPI.createProfile('New Profile', { key: 'value' }, 'Description');

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/admin/profiles/',
        expect.objectContaining({ method: 'POST' }),
      );
      expect(parseRequiredAPIResponse).toHaveBeenCalledWith(
        mockResponse,
        'Failed to create profile: 200',
        'Failed to parse created profile',
      );
    });

    it('throws on failure', async () => {
      const mockResponse = { ok: false, status: 400 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(parseRequiredAPIResponse).mockRejectedValueOnce(new Error('Bad request'));

      await expect(AgentProfilesAPI.createProfile('Name', {})).rejects.toThrow('Bad request');
    });

    it('fails closed when the created profile payload is malformed', async () => {
      const mockResponse = { ok: true, status: 200 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce({ id: 'p1', name: 'Bad' } as any);

      await expect(AgentProfilesAPI.createProfile('Name', {})).rejects.toThrow(
        INVALID_AGENT_PROFILE_RESPONSE_MESSAGE,
      );
    });
  });

  describe('updateProfile', () => {
    it('fails closed when the updated profile payload is malformed', async () => {
      const mockResponse = { ok: true, status: 200 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce({ id: 'p1', name: 'Bad' } as any);

      await expect(AgentProfilesAPI.updateProfile('p1', 'Name', {})).rejects.toThrow(
        INVALID_AGENT_PROFILE_RESPONSE_MESSAGE,
      );
    });
  });

  describe('deleteProfile', () => {
    it('deletes a profile', async () => {
      const mockResponse = { ok: true, status: 204 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);

      await AgentProfilesAPI.deleteProfile('p1');

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/admin/profiles/p1',
        expect.objectContaining({ method: 'DELETE' }),
      );
      expect(assertAPIResponseOKOrAllowedStatus).toHaveBeenCalledWith(
        mockResponse,
        204,
        'Failed to delete profile: 204',
      );
    });

    it('treats canonical 204 delete responses as success even when ok is false-like', async () => {
      const mockResponse = { ok: false, status: 204 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);

      await expect(AgentProfilesAPI.deleteProfile('p1')).resolves.toBeUndefined();
      expect(assertAPIResponseOKOrAllowedStatus).toHaveBeenCalledWith(
        mockResponse,
        204,
        'Failed to delete profile: 204',
      );
    });
  });

  describe('listAssignments', () => {
    it('fetches all assignments', async () => {
      const mockAssignments: AgentProfileAssignment[] = [
        { agent_id: 'a1', profile_id: 'p1', updated_at: '' },
      ];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockAssignments);

      const result = await AgentProfilesAPI.listAssignments();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/admin/profiles/assignments');
      expect(withAPIErrorStatusFallback).toHaveBeenCalledWith(expect.any(Promise), 402, []);
      expect(result).toEqual(mockAssignments);
    });

    it('returns empty array on 402', async () => {
      vi.mocked(apiFetchJSON).mockRejectedValueOnce(Object.assign(new Error('Payment Required'), { status: 402 }));

      const result = await AgentProfilesAPI.listAssignments();

      expect(result).toEqual([]);
    });

    it('fails closed when the assignments response is malformed', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ assignments: [] } as any);

      await expect(AgentProfilesAPI.listAssignments()).rejects.toThrow(
        INVALID_AGENT_PROFILE_ASSIGNMENT_LIST_MESSAGE,
      );
    });

    it('fails closed when an assignment entry is malformed', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce([{ agent_id: 'a1', profile_id: 3 }] as any);

      await expect(AgentProfilesAPI.listAssignments()).rejects.toThrow(
        INVALID_AGENT_PROFILE_ASSIGNMENT_LIST_MESSAGE,
      );
    });
  });

  describe('assignProfile', () => {
    it('assigns profile to agent', async () => {
      const mockResponse = { ok: true } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);

      await AgentProfilesAPI.assignProfile('agent-1', 'profile-1');

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/admin/profiles/assignments',
        expect.objectContaining({ method: 'POST' }),
      );
    });

    it('throws the canonical missing-profile message on 404', async () => {
      const mockResponse = { ok: false, status: 404 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(assertAPIResponseOKOrThrowStatus).mockRejectedValueOnce(
        new Error(MISSING_AGENT_PROFILE_ASSIGNMENT_MESSAGE),
      );

      await expect(AgentProfilesAPI.assignProfile('agent-1', 'profile-1')).rejects.toThrow(
        MISSING_AGENT_PROFILE_ASSIGNMENT_MESSAGE,
      );
      expect(assertAPIResponseOKOrThrowStatus).toHaveBeenCalledWith(
        mockResponse,
        404,
        MISSING_AGENT_PROFILE_ASSIGNMENT_MESSAGE,
        'Failed to assign profile: 404',
      );
    });
  });

  describe('unassignProfile', () => {
    it('removes profile assignment', async () => {
      const mockResponse = { ok: true, status: 204 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);

      await AgentProfilesAPI.unassignProfile('agent-1');

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/admin/profiles/assignments/agent-1',
        expect.objectContaining({ method: 'DELETE' }),
      );
      expect(assertAPIResponseOKOrAllowedStatus).toHaveBeenCalledWith(
        mockResponse,
        204,
        'Failed to unassign profile: 204',
      );
    });

    it('treats canonical 204 unassign responses as success even when ok is false-like', async () => {
      const mockResponse = { ok: false, status: 204 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);

      await expect(AgentProfilesAPI.unassignProfile('agent-1')).resolves.toBeUndefined();
      expect(assertAPIResponseOKOrAllowedStatus).toHaveBeenCalledWith(
        mockResponse,
        204,
        'Failed to unassign profile: 204',
      );
    });
  });

  describe('suggestProfile', () => {
    it('gets AI suggestion', async () => {
      const mockResponse = {
        ok: true,
        status: 200,
        text: () =>
          Promise.resolve(JSON.stringify({ name: 'Suggested', description: '', config: {}, rationale: [] })),
      } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce({
        name: 'Suggested',
        description: '',
        config: {},
        rationale: [],
      });

      await AgentProfilesAPI.suggestProfile({ prompt: 'Create a profile' });

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/admin/profiles/suggestions',
        expect.objectContaining({ method: 'POST' }),
      );
      expect(parseRequiredAPIResponse).toHaveBeenCalledWith(
        mockResponse,
        'Failed to get suggestion: 200',
        'Failed to parse profile suggestion',
      );
    });

    it('throws on 503', async () => {
      const mockResponse = { ok: false, status: 503 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(assertAPIResponseOKOrThrowStatus).mockRejectedValueOnce(
        new Error('Pulse Assistant service is not available. Please check Pulse Assistant settings.'),
      );

      await expect(AgentProfilesAPI.suggestProfile({ prompt: 'test' })).rejects.toThrow(
        'Pulse Assistant service is not available',
      );
      expect(assertAPIResponseOKOrThrowStatus).toHaveBeenCalledWith(
        mockResponse,
        503,
        'Pulse Assistant service is not available. Please check Pulse Assistant settings.',
        'Failed to get suggestion: 503',
      );
    });

    it('does not infer service-unavailable status from arbitrary responses', async () => {
      const mockResponse = { ok: false, status: 500 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(assertAPIResponseOKOrThrowStatus).mockRejectedValueOnce(new Error('Server error'));

      await expect(AgentProfilesAPI.suggestProfile({ prompt: 'test' })).rejects.toThrow(
        'Server error',
      );
      expect(assertAPIResponseOKOrThrowStatus).toHaveBeenCalledWith(
        mockResponse,
        503,
        'Pulse Assistant service is not available. Please check Pulse Assistant settings.',
        'Failed to get suggestion: 500',
      );
    });

    it('fails closed when the suggestion success payload is invalid JSON', async () => {
      const mockResponse = { ok: true, status: 200 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(parseRequiredAPIResponse).mockRejectedValueOnce(
        new Error('Failed to parse profile suggestion'),
      );

      await expect(AgentProfilesAPI.suggestProfile({ prompt: 'test' })).rejects.toThrow(
        'Failed to parse profile suggestion',
      );
    });

    it('fails closed when the suggestion payload shape is malformed', async () => {
      const mockResponse = { ok: true, status: 200 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(parseRequiredAPIResponse).mockResolvedValueOnce({
        name: 'Suggested',
        description: '',
        config: [],
        rationale: [],
      } as any);

      await expect(AgentProfilesAPI.suggestProfile({ prompt: 'test' })).rejects.toThrow(
        INVALID_AGENT_PROFILE_SUGGESTION_MESSAGE,
      );
    });
  });

  describe('getConfigSchema', () => {
    it('fetches and transforms schema', async () => {
      const mockSchema = [
        { Key: 'cpu_limit', Type: 'number', Description: 'CPU limit', Default: 4, Required: true },
      ];
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response(JSON.stringify(mockSchema), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      );

      const result = await AgentProfilesAPI.getConfigSchema();

      expect(apiFetch).toHaveBeenCalledWith('/api/admin/profiles/schema');
      expect(assertAPIResponseOK).toHaveBeenCalledWith(expect.any(Response), 'Failed to fetch profile schema');
      expect(result).toEqual([
        {
          key: 'cpu_limit',
          type: 'number',
          description: 'CPU limit',
          defaultValue: 4,
          required: true,
        },
      ]);
    });

    it('fails closed when schema payload is malformed', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response(JSON.stringify({ schema: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      );

      await expect(AgentProfilesAPI.getConfigSchema()).rejects.toThrow(
        INVALID_AGENT_PROFILE_SCHEMA_MESSAGE,
      );
    });

    it('fails closed when a schema definition is malformed', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response(
          JSON.stringify([
            { Key: 'cpu_limit', Type: 'number', Description: 'CPU limit', Required: 'yes' },
          ]),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      );

      await expect(AgentProfilesAPI.getConfigSchema()).rejects.toThrow(
        INVALID_AGENT_PROFILE_SCHEMA_MESSAGE,
      );
    });
  });

  describe('validateConfig', () => {
    it('returns validation result', async () => {
      const mockResult = { Valid: true, Errors: [], Warnings: [] };
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response(JSON.stringify(mockResult), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      );

      const result = await AgentProfilesAPI.validateConfig({ cpu_limit: 4 });

      expect(apiFetch).toHaveBeenCalledWith('/api/admin/profiles/validate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ cpu_limit: 4 }),
      });
      expect(assertAPIResponseOK).toHaveBeenCalledWith(
        expect.any(Response),
        'Failed to validate profile config',
      );
      expect(result.valid).toBe(true);
    });

    it('fails closed when the validation response is null', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response('null', {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      );

      await expect(AgentProfilesAPI.validateConfig({})).rejects.toThrow(
        INVALID_AGENT_PROFILE_VALIDATION_MESSAGE,
      );
    });

    it('fails closed when validation arrays are malformed', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            Valid: false,
            Errors: 'bad',
            Warnings: null,
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      );

      await expect(AgentProfilesAPI.validateConfig({})).rejects.toThrow(
        INVALID_AGENT_PROFILE_VALIDATION_MESSAGE,
      );
    });

    it('fails closed when a validation item is malformed', async () => {
      vi.mocked(apiFetch).mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            Valid: false,
            Errors: [{ Key: 'cpu_limit', Message: 1 }],
            Warnings: [],
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      );

      await expect(AgentProfilesAPI.validateConfig({})).rejects.toThrow(
        INVALID_AGENT_PROFILE_VALIDATION_MESSAGE,
      );
    });
  });

  it('does not infer API status from raw error message text', async () => {
    vi.mocked(apiFetchJSON).mockRejectedValueOnce(new Error('402'));

    await expect(AgentProfilesAPI.listProfiles()).rejects.toThrow('402');
  });
});
