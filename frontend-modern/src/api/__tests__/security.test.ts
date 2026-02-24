import { describe, expect, it, vi, beforeEach } from 'vitest';
import { SecurityAPI, type APITokenRecord, type CreateAPITokenResponse } from '../security';
import { apiFetchJSON } from '@/utils/apiClient';
import type { SecurityStatus } from '@/types/config';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('SecurityAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getStatus', () => {
    it('fetches security status', async () => {
      const mockStatus: SecurityStatus = {
        hasAuthentication: true,
        apiTokenConfigured: true,
        apiTokenHint: 'pmp_***',
        requiresAuth: true,
        credentialsEncrypted: true,
        exportProtected: true,
        hasAuditLogging: true,
        configuredButPendingRestart: false,
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockStatus);

      const result = await SecurityAPI.getStatus();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/security/status');
      expect(result).toEqual(mockStatus);
    });
  });

  describe('listTokens', () => {
    it('fetches all tokens', async () => {
      const mockTokens: APITokenRecord[] = [
        { id: 't1', name: 'Token 1', prefix: 'pmp_', suffix: 'abc', createdAt: '' },
      ];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ tokens: mockTokens });

      const result = await SecurityAPI.listTokens();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/security/tokens');
      expect(result).toEqual(mockTokens);
    });

    it('returns empty array when tokens is null', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ tokens: null });

      const result = await SecurityAPI.listTokens();

      expect(result).toEqual([]);
    });
  });

  describe('createToken', () => {
    it('creates token with name and scopes', async () => {
      const mockResponse: CreateAPITokenResponse = {
        token: 'pmp_xxx',
        record: { id: 't1', name: 'Token', prefix: 'pmp_', suffix: 'xxx', createdAt: '' },
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockResponse);

      const result = await SecurityAPI.createToken('My Token', ['read', 'write']);

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/security/tokens',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ name: 'My Token', scopes: ['read', 'write'] }),
        }),
      );
      expect(result).toEqual(mockResponse);
    });

    it('creates token without optional params', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({
        token: 'pmp_xxx',
        record: {},
      } as CreateAPITokenResponse);

      await SecurityAPI.createToken();

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/security/tokens',
        expect.objectContaining({
          body: JSON.stringify({}),
        }),
      );
    });
  });

  describe('deleteToken', () => {
    it('deletes a token', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(undefined);

      await SecurityAPI.deleteToken('token-1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/security/tokens/token-1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });

    it('encodes special characters in token id', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(undefined);

      await SecurityAPI.deleteToken('token/1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/security/tokens/token%2F1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });
  });
});
