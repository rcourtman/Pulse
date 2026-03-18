import { describe, expect, it, vi, beforeEach } from 'vitest';
import { AgentMetadataAPI, type AgentMetadata } from '../agentMetadata';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('AgentMetadataAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getMetadata', () => {
    it('fetches metadata for a specific agent', async () => {
      const mockMetadata: AgentMetadata = {
        id: 'agent-1',
        customUrl: 'https://example.com',
        description: 'Test agent',
        tags: ['production', 'web'],
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockMetadata);

      const result = await AgentMetadataAPI.getMetadata('agent-1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/agents/metadata/agent-1');
      expect(result).toEqual(mockMetadata);
    });

    it('encodes special characters in agentId', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ id: 'agent+1' });

      await AgentMetadataAPI.getMetadata('agent+1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/agents/metadata/agent%2B1');
    });
  });

  describe('getAllMetadata', () => {
    it('fetches all agent metadata', async () => {
      const mockAllMetadata: Record<string, AgentMetadata> = {
        'agent-1': { id: 'agent-1', description: 'Agent 1' },
        'agent-2': { id: 'agent-2', description: 'Agent 2' },
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockAllMetadata);

      const result = await AgentMetadataAPI.getAllMetadata();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/agents/metadata');
      expect(result).toEqual(mockAllMetadata);
    });
  });

  describe('updateMetadata', () => {
    it('updates metadata for an agent', async () => {
      const updatedMetadata: AgentMetadata = {
        id: 'agent-1',
        description: 'Updated description',
        tags: ['updated'],
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(updatedMetadata);

      const result = await AgentMetadataAPI.updateMetadata('agent-1', {
        description: 'Updated description',
        tags: ['updated'],
      });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/agents/metadata/agent-1',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ description: 'Updated description', tags: ['updated'] }),
        }),
      );
      expect(result).toEqual(updatedMetadata);
    });
  });

  describe('deleteMetadata', () => {
    it('deletes metadata for an agent', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(undefined);

      await AgentMetadataAPI.deleteMetadata('agent-1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/agents/metadata/agent-1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });
  });
});
