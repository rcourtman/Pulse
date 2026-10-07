import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';

import type { APITokenRecord } from '@/api/security';
import { DOCKER_REPORT_SCOPE } from '@/constants/apiScopes';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { APITokenManager } from '../APITokenManager';

const listTokensMock = vi.fn();

vi.mock('@/api/security', () => ({
  SecurityAPI: {
    listTokens: (...args: unknown[]) => listTokensMock(...args),
    createToken: vi.fn(),
    updateTokenScopes: vi.fn(),
    renameToken: vi.fn(),
    deleteToken: vi.fn(),
  },
}));

vi.mock('@/api/agentCapabilities', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/agentCapabilities')>();
  return {
    ...actual,
    // The manifest only feeds token presets; leave it pending.
    fetchAgentCapabilitiesManifest: vi.fn(() => new Promise(() => {})),
  };
});

vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('@/stores/tokenReveal', () => ({
  showTokenReveal: vi.fn(),
  useTokenRevealState: () => () => null,
}));

vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), debug: vi.fn(), warn: vi.fn() },
}));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    markDockerRuntimesTokenRevoked: vi.fn(),
    markAgentsTokenRevoked: vi.fn(),
  }),
}));

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({ resources: () => [], byType: () => [] }),
}));

const token: APITokenRecord = {
  id: 'token-1',
  name: 'Runtime token',
  prefix: 'pulse',
  suffix: '1234',
  createdAt: '2026-03-12T10:00:00.000Z',
  lastUsedAt: '2026-03-12T10:03:00.000Z',
  scopes: [DOCKER_REPORT_SCOPE],
};

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('APITokenManager relative ages', () => {
  it('keeps a token Created age moving while Last used stays the age at read time', async () => {
    vi.useFakeTimers({
      toFake: ['Date', 'setInterval', 'clearInterval'],
      now: Date.parse('2026-03-12T10:05:00Z'),
    });
    listTokensMock.mockResolvedValue([token]);

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    expect((await screen.findAllByText('5 mins ago')).length).toBeGreaterThan(0);
    expect(screen.getAllByText('2 mins ago').length).toBeGreaterThan(0);

    // The token list is not re-read: only the clock moves.
    vi.advanceTimersByTime(3 * 60 * 2 * RELATIVE_TIME_TICK_MS);

    expect(screen.getAllByText('3 hours ago').length).toBeGreaterThan(0);
    expect(screen.queryAllByText('5 mins ago')).toHaveLength(0);
    // Last used is a latest reading from a list read once, so it keeps the
    // age it had when the list was read.
    expect(screen.getAllByText('2 mins ago').length).toBeGreaterThan(0);
  });
});
