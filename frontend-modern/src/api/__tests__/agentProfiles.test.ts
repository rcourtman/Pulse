import { describe, expect, it, vi, beforeEach } from 'vitest';
import { AgentProfilesAPI, type AgentProfile, type AgentProfileAssignment } from '../agentProfiles';
import { apiFetchJSON, apiFetch } from '@/utils/apiClient';
import { readAPIErrorMessage } from '../responseUtils';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
  apiFetch: vi.fn(),
}));

vi.mock('../responseUtils', () => ({
  readAPIErrorMessage: vi.fn().mockResolvedValue('Error'),
}));

describe('AgentProfilesAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('listProfiles', () => {
    it('fetches all profiles', async () => {
      const mockProfiles: AgentProfile[] = [
        { id: 'p1', name: 'Profile 1', config: {}, created_at: '', updated_at: '' },
      ];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockProfiles);

      const result = await AgentProfilesAPI.listProfiles();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/admin/profiles/');
      expect(result).toEqual(mockProfiles);
    });

    it('returns empty array on 402 error', async () => {
      vi.mocked(apiFetchJSON).mockRejectedValueOnce(new Error('402'));

      const result = await AgentProfilesAPI.listProfiles();

      expect(result).toEqual([]);
    });

    it('returns empty array when response is null', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(null);

      const result = await AgentProfilesAPI.listProfiles();

      expect(result).toEqual([]);
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
      expect(result).toEqual(mockProfile);
    });

    it('returns null on 404', async () => {
      vi.mocked(apiFetchJSON).mockRejectedValueOnce(new Error('404'));

      const result = await AgentProfilesAPI.getProfile('p1');

      expect(result).toBeNull();
    });
  });

  describe('createProfile', () => {
    it('creates a new profile', async () => {
      const mockResponse = {
        ok: true,
        json: () => Promise.resolve({ id: 'p1' }),
      } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);

      await AgentProfilesAPI.createProfile('New Profile', { key: 'value' }, 'Description');

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/admin/profiles/',
        expect.objectContaining({ method: 'POST' }),
      );
    });

    it('throws on failure', async () => {
      const mockResponse = { ok: false, status: 400 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);
      vi.mocked(readAPIErrorMessage).mockResolvedValueOnce('Bad request');

      await expect(AgentProfilesAPI.createProfile('Name', {})).rejects.toThrow('Bad request');
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
      expect(result).toEqual(mockAssignments);
    });

    it('returns empty array on 402', async () => {
      vi.mocked(apiFetchJSON).mockRejectedValueOnce(new Error('402'));

      const result = await AgentProfilesAPI.listAssignments();

      expect(result).toEqual([]);
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
    });
  });

  describe('suggestProfile', () => {
    it('gets AI suggestion', async () => {
      const mockResponse = {
        ok: true,
        json: () =>
          Promise.resolve({ name: 'Suggested', description: '', config: {}, rationale: [] }),
      } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);

      await AgentProfilesAPI.suggestProfile({ prompt: 'Create a profile' });

      expect(apiFetch).toHaveBeenCalledWith(
        '/api/admin/profiles/suggestions',
        expect.objectContaining({ method: 'POST' }),
      );
    });

    it('throws on 503', async () => {
      const mockResponse = { ok: false, status: 503 } as unknown as Response;
      vi.mocked(apiFetch).mockResolvedValueOnce(mockResponse);

      await expect(AgentProfilesAPI.suggestProfile({ prompt: 'test' })).rejects.toThrow(
        'Pulse Assistant service is not available',
      );
    });
  });

  describe('getConfigSchema', () => {
    it('fetches and transforms schema', async () => {
      const mockSchema = [
        { Key: 'cpu_limit', Type: 'number', Description: 'CPU limit', Default: 4, Required: true },
      ];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockSchema);

      const result = await AgentProfilesAPI.getConfigSchema();

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
  });

  describe('validateConfig', () => {
    it('returns validation result', async () => {
      const mockResult = { Valid: true, Errors: [], Warnings: [] };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockResult);

      const result = await AgentProfilesAPI.validateConfig({ cpu_limit: 4 });

      expect(result.valid).toBe(true);
    });

    it('returns valid true when response is null', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(null);

      const result = await AgentProfilesAPI.validateConfig({});

      expect(result.valid).toBe(true);
    });
  });
});
