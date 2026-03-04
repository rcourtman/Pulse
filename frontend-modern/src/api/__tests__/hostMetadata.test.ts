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
    it('fetches metadata for a specific host', async () => {
      const mockMetadata: AgentMetadata = {
        id: 'host-1',
        customUrl: 'https://example.com',
        description: 'Test host',
        tags: ['production', 'web'],
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockMetadata);

      const result = await AgentMetadataAPI.getMetadata('host-1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/agents/metadata/host-1');
      expect(result).toEqual(mockMetadata);
    });

    it('encodes special characters in hostId', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ id: 'host+1' });

      await AgentMetadataAPI.getMetadata('host+1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/agents/metadata/host%2B1');
    });
  });

  describe('getAllMetadata', () => {
    it('fetches all host metadata', async () => {
      const mockAllMetadata: Record<string, AgentMetadata> = {
        'host-1': { id: 'host-1', description: 'Host 1' },
        'host-2': { id: 'host-2', description: 'Host 2' },
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockAllMetadata);

      const result = await AgentMetadataAPI.getAllMetadata();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/agents/metadata');
      expect(result).toEqual(mockAllMetadata);
    });
  });

  describe('updateMetadata', () => {
    it('updates metadata for a host', async () => {
      const updatedMetadata: AgentMetadata = {
        id: 'host-1',
        description: 'Updated description',
        tags: ['updated'],
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(updatedMetadata);

      const result = await AgentMetadataAPI.updateMetadata('host-1', {
        description: 'Updated description',
        tags: ['updated'],
      });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/agents/metadata/host-1',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ description: 'Updated description', tags: ['updated'] }),
        }),
      );
      expect(result).toEqual(updatedMetadata);
    });
  });

  describe('deleteMetadata', () => {
    it('deletes metadata for a host', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(undefined);

      await AgentMetadataAPI.deleteMetadata('host-1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/agents/metadata/host-1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });
  });
});
