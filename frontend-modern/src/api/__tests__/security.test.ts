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
        sessionCapabilities: {
          demoMode: false,
          assistantEnabled: true,
        },
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockStatus);

      const result = await SecurityAPI.getStatus();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/security/status');
      expect(result).toEqual(mockStatus);
    });

    it('preserves assistant availability in session capabilities', async () => {
      const mockStatus: SecurityStatus = {
        hasAuthentication: true,
        apiTokenConfigured: false,
        requiresAuth: true,
        credentialsEncrypted: true,
        exportProtected: true,
        hasAuditLogging: false,
        configuredButPendingRestart: false,
        sessionCapabilities: {
          demoMode: false,
          assistantEnabled: true,
        },
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockStatus);

      const result = await SecurityAPI.getStatus();

      expect(result.sessionCapabilities?.assistantEnabled).toBe(true);
    });
  });

  describe('listTokens', () => {
    it('fetches all tokens', async () => {
      const mockTokens: APITokenRecord[] = [
        {
          id: 't1',
          name: 'Token 1',
          prefix: 'pmp_',
          suffix: 'abc',
          createdAt: '',
          ownerUserId: 'owner@example.com',
        },
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

    it('returns empty array when tokens is malformed', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ tokens: 'bad' } as any);

      const result = await SecurityAPI.listTokens();

      expect(result).toEqual([]);
    });
  });

  describe('getToken', () => {
    it('fetches a single token record', async () => {
      const mockRecord: APITokenRecord = {
        id: 't1',
        name: 'Token 1',
        prefix: 'pmp_',
        suffix: 'abc',
        createdAt: '',
        lastUsedAt: '2026-03-24T12:00:00Z',
        expiresAt: '2026-03-25T12:00:00Z',
        ownerUserId: 'owner@example.com',
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ record: mockRecord });

      const result = await SecurityAPI.getToken('token/1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/security/tokens/token%2F1');
      expect(result).toEqual(mockRecord);
    });
  });

  describe('createToken', () => {
    it('creates token with name and scopes', async () => {
      const mockResponse: CreateAPITokenResponse = {
        token: 'pmp_xxx',
        record: {
          id: 't1',
          name: 'Token',
          prefix: 'pmp_',
          suffix: 'xxx',
          createdAt: '',
          ownerUserId: 'owner@example.com',
        },
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

    it('preserves owner user binding from the API response', async () => {
      const mockResponse: CreateAPITokenResponse = {
        token: 'pmp_xxx',
        record: {
          id: 't1',
          name: 'Token',
          prefix: 'pmp_',
          suffix: 'xxx',
          createdAt: '',
          ownerUserId: 'owner@example.com',
        },
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockResponse);

      const result = await SecurityAPI.createToken('My Token', ['read']);

      expect(result.record.ownerUserId).toBe('owner@example.com');
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

  describe('createRelayMobileAccessToken', () => {
    it('creates a server-owned relay mobile runtime token', async () => {
      const mockResponse: CreateAPITokenResponse = {
        token: 'pmp_mobile',
        record: {
          id: 'relay-mobile-1',
          name: 'Pulse Mobile relay access 2026-03-24T23:30:00Z',
          prefix: 'pmp_',
          suffix: 'bile',
          createdAt: '',
          scopes: ['relay:mobile:access'],
          ownerUserId: 'owner@example.com',
        },
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockResponse);

      const result = await SecurityAPI.createRelayMobileAccessToken();

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/security/tokens/relay-mobile',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({}),
        }),
      );
      expect(result).toEqual(mockResponse);
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
