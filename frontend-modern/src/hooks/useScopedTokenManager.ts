import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import type { APITokenRecord, CreateAPITokenResponse } from '@/api/security';
import { SecurityAPI } from '@/api/security';

export interface UseScopedTokenManagerOptions {
  scope: string | readonly string[];
  storageKey: string;
  legacyKeys?: readonly string[];
}

export interface ScopedTokenManager {
  token: () => string | null;
  setToken: (value: string | null) => void;
  hasStoredToken: () => boolean;
  availableTokens: () => APITokenRecord[];
  loadingTokens: () => boolean;
  tokensLoaded: () => boolean;
  tokensError: () => boolean;
  tokenAccessDenied: () => boolean;
  loadTokens: () => Promise<void>;
  retryLoadTokens: () => Promise<void>;
  isGeneratingToken: () => boolean;
  generateToken: (name: string) => Promise<CreateAPITokenResponse>;
}

const tokenMatchesScopes = (token: APITokenRecord, requiredScopes: string[]): boolean => {
  const scopes = token.scopes ?? [];
  if (scopes.length === 0) {
    // Legacy tokens without scopes behave as wildcard.
    return true;
  }
  if (scopes.includes('*')) {
    return true;
  }
  return requiredScopes.every((scope) => scopes.includes(scope));
};

const readStoredToken = (primaryKey: string, legacyKeys: readonly string[] = []) => {
  if (typeof window === 'undefined') {
    return null;
  }
  const primary = window.localStorage.getItem(primaryKey);
  if (primary) {
    return primary;
  }
  for (const key of legacyKeys) {
    const value = window.localStorage.getItem(key);
    if (value) {
      return value;
    }
  }
  return null;
};

export const useScopedTokenManager = (options: UseScopedTokenManagerOptions): ScopedTokenManager => {
  const requiredScopes = Array.isArray(options.scope) ? [...options.scope] : [options.scope];
  const { storageKey, legacyKeys = [] } = options;

  const [token, setTokenState] = createSignal<string | null>(readStoredToken(storageKey, legacyKeys));
  const [availableTokens, setAvailableTokens] = createSignal<APITokenRecord[]>([]);
  const [loadingTokens, setLoadingTokens] = createSignal(false);
  const [tokensLoaded, setTokensLoaded] = createSignal(false);
  const [tokensError, setTokensError] = createSignal(false);
  const [tokenAccessDenied, setTokenAccessDenied] = createSignal(false);
  const [isGeneratingToken, setIsGeneratingToken] = createSignal(false);

  createEffect(() => {
    const current = token();
    if (typeof window === 'undefined') {
      return;
    }
    try {
      if (current) {
        window.localStorage.setItem(storageKey, current);
        try {
          window.dispatchEvent(new StorageEvent('storage', { key: storageKey, newValue: current }));
        } catch (eventErr) {
          console.debug('Unable to dispatch storage event for token update', eventErr);
        }
      } else {
        window.localStorage.removeItem(storageKey);
        try {
          window.dispatchEvent(new StorageEvent('storage', { key: storageKey, newValue: null }));
        } catch (eventErr) {
          console.debug('Unable to dispatch storage event for token removal', eventErr);
        }
      }
    } catch (err) {
      console.warn('Unable to persist API token in localStorage', err);
    }
  });

  if (typeof window !== 'undefined') {
    const handleStorage = (event: StorageEvent) => {
      if (!event.key) return;
      if (event.key === storageKey || legacyKeys.includes(event.key)) {
        const updated = readStoredToken(storageKey, legacyKeys);
        setTokenState(updated);
      }
    };
    window.addEventListener('storage', handleStorage);
    onCleanup(() => window.removeEventListener('storage', handleStorage));
  }

  const loadTokens = async () => {
    if (loadingTokens()) return;
    setLoadingTokens(true);
    setTokensError(false);
    setTokensLoaded(false);
    try {
      const tokens = await SecurityAPI.listTokens();
      const filtered = tokens.filter((candidate) => tokenMatchesScopes(candidate, requiredScopes));
      filtered.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());
      setAvailableTokens(filtered);
      setTokenAccessDenied(false);
      setTokensLoaded(true);
    } catch (err) {
      setTokensError(true);
      if (err instanceof Error && /authentication required/i.test(err.message)) {
        setTokenAccessDenied(true);
      }
      throw err;
    } finally {
      setLoadingTokens(false);
    }
  };

  const retryLoadTokens = async () => {
    setTokensLoaded(false);
    await loadTokens();
  };

  const generateToken = async (name: string) => {
    if (isGeneratingToken()) {
      throw new Error('Token generation already in progress');
    }
    setIsGeneratingToken(true);
    try {
      const response = await SecurityAPI.createToken(name, requiredScopes);
      const { token: newToken, record } = response;
      setTokenState(newToken);
      setAvailableTokens((current) => {
        const filtered = current.filter((item) => item.id !== record.id);
        return [record, ...filtered];
      });
      setTokenAccessDenied(false);
      return response;
    } catch (err) {
      if (err instanceof Error && /authentication required|forbidden/i.test(err.message)) {
        setTokenAccessDenied(true);
      }
      throw err;
    } finally {
      setIsGeneratingToken(false);
    }
  };

  const hasStoredToken = createMemo(() => Boolean(token()));

  return {
    token,
    setToken: setTokenState,
    hasStoredToken,
    availableTokens,
    loadingTokens,
    tokensLoaded,
    tokensError,
    tokenAccessDenied,
    loadTokens,
    retryLoadTokens,
    isGeneratingToken,
    generateToken,
  };
};
