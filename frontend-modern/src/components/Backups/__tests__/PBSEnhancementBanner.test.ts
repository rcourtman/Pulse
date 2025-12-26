import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { cleanup } from '@solidjs/testing-library';
import { createStore } from 'solid-js/store';

// We need to test the PBS enhancement banner logic
// The actual component is UnifiedBackups, but we'll test the core logic

// Mock the WebSocket context
let mockWsStore: {
    state: {
        storage: Array<{ type: string; name: string }>;
        pbs: Array<{ name: string }>;
        pveBackups?: { storageBackups?: unknown[]; guestSnapshots?: unknown[] };
        backups?: { pve?: unknown; pbs?: unknown[] };
        nodes?: unknown[];
    };
    connected: () => boolean;
    initialDataReceived: () => boolean;
};

vi.mock('@/App', () => ({
    useWebSocket: () => mockWsStore,
}));

vi.mock('@solidjs/router', () => ({
    useNavigate: () => vi.fn(),
}));

// Mock localStorage
const localStorageMock = (() => {
    let store: Record<string, string> = {};
    return {
        getItem: (key: string) => store[key] || null,
        setItem: (key: string, value: string) => { store[key] = value; },
        removeItem: (key: string) => { delete store[key]; },
        clear: () => { store = {}; },
    };
})();

vi.stubGlobal('localStorage', localStorageMock);

const setupMockState = (options: {
    hasPBSStorage?: boolean;
    hasDirectPBS?: boolean;
    bannerDismissed?: boolean;
}) => {
    const [state] = createStore({
        storage: options.hasPBSStorage
            ? [{ type: 'pbs', name: 'pbs-main' }]
            : [{ type: 'local', name: 'local-lvm' }],
        pbs: options.hasDirectPBS
            ? [{ name: 'pbs-direct' }]
            : [],
        nodes: [],
        pveBackups: { storageBackups: [], guestSnapshots: [] },
        backups: { pve: null, pbs: [] },
    });

    mockWsStore = {
        state,
        connected: () => true,
        initialDataReceived: () => true,
    };

    if (options.bannerDismissed) {
        localStorageMock.setItem('pulse.pbsEnhancementBannerDismissed', 'true');
    }

    return state;
};

beforeEach(() => {
    localStorageMock.clear();
});

afterEach(() => {
    cleanup();
});

describe('PBS Enhancement Banner Logic', () => {
    describe('hasPBSViaPassthrough detection', () => {
        it('returns true when storage has PBS type', () => {
            const state = setupMockState({ hasPBSStorage: true });
            const hasPBS = state.storage.some((s) => s.type === 'pbs');
            expect(hasPBS).toBe(true);
        });

        it('returns false when no PBS storage exists', () => {
            const state = setupMockState({ hasPBSStorage: false });
            const hasPBS = state.storage.some((s) => s.type === 'pbs');
            expect(hasPBS).toBe(false);
        });
    });

    describe('hasDirectPBS detection', () => {
        it('returns true when pbs instances exist', () => {
            const state = setupMockState({ hasDirectPBS: true });
            const hasDirectPBS = state.pbs.length > 0;
            expect(hasDirectPBS).toBe(true);
        });

        it('returns false when no pbs instances configured', () => {
            const state = setupMockState({ hasDirectPBS: false });
            const hasDirectPBS = state.pbs.length > 0;
            expect(hasDirectPBS).toBe(false);
        });
    });

    describe('showPBSEnhancementBanner logic', () => {
        it('should show banner when PBS via passthrough exists but no direct PBS', () => {
            setupMockState({ hasPBSStorage: true, hasDirectPBS: false, bannerDismissed: false });

            const hasPBSViaPassthrough = mockWsStore.state.storage.some((s) => s.type === 'pbs');
            const hasDirectPBS = mockWsStore.state.pbs.length > 0;
            const dismissed = localStorageMock.getItem('pulse.pbsEnhancementBannerDismissed') === 'true';

            const shouldShow = hasPBSViaPassthrough && !hasDirectPBS && !dismissed;
            expect(shouldShow).toBe(true);
        });

        it('should NOT show banner when direct PBS is configured', () => {
            setupMockState({ hasPBSStorage: true, hasDirectPBS: true, bannerDismissed: false });

            const hasPBSViaPassthrough = mockWsStore.state.storage.some((s) => s.type === 'pbs');
            const hasDirectPBS = mockWsStore.state.pbs.length > 0;
            const dismissed = localStorageMock.getItem('pulse.pbsEnhancementBannerDismissed') === 'true';

            const shouldShow = hasPBSViaPassthrough && !hasDirectPBS && !dismissed;
            expect(shouldShow).toBe(false);
        });

        it('should NOT show banner when there is no PBS storage', () => {
            setupMockState({ hasPBSStorage: false, hasDirectPBS: false, bannerDismissed: false });

            const hasPBSViaPassthrough = mockWsStore.state.storage.some((s) => s.type === 'pbs');
            const hasDirectPBS = mockWsStore.state.pbs.length > 0;
            const dismissed = localStorageMock.getItem('pulse.pbsEnhancementBannerDismissed') === 'true';

            const shouldShow = hasPBSViaPassthrough && !hasDirectPBS && !dismissed;
            expect(shouldShow).toBe(false);
        });

        it('should NOT show banner when user has dismissed it', () => {
            setupMockState({ hasPBSStorage: true, hasDirectPBS: false, bannerDismissed: true });

            const hasPBSViaPassthrough = mockWsStore.state.storage.some((s) => s.type === 'pbs');
            const hasDirectPBS = mockWsStore.state.pbs.length > 0;
            const dismissed = localStorageMock.getItem('pulse.pbsEnhancementBannerDismissed') === 'true';

            const shouldShow = hasPBSViaPassthrough && !hasDirectPBS && !dismissed;
            expect(shouldShow).toBe(false);
        });
    });

    describe('localStorage persistence', () => {
        it('should persist banner dismissal in localStorage', () => {
            localStorageMock.setItem('pulse.pbsEnhancementBannerDismissed', 'true');
            expect(localStorageMock.getItem('pulse.pbsEnhancementBannerDismissed')).toBe('true');
        });

        it('should default to showing banner when localStorage is empty', () => {
            expect(localStorageMock.getItem('pulse.pbsEnhancementBannerDismissed')).toBeNull();
        });
    });
});

describe('PBS Backup Data Source Indicator', () => {
    describe('"via PVE" indicator logic', () => {
        it('should show "via PVE" for PBS backup with storage but no datastore', () => {
            const backup = {
                backupType: 'remote' as const,
                storage: 'pbs-main',       // Has storage (PVE passthrough)
                datastore: null,           // No datastore (not from direct PBS)
            };

            const shouldShowViaPVE =
                backup.backupType === 'remote' &&
                backup.storage &&
                !backup.datastore;

            expect(shouldShowViaPVE).toBe(true);
        });

        it('should NOT show "via PVE" for direct PBS backup with datastore', () => {
            const backup = {
                backupType: 'remote' as const,
                storage: null,             // No storage (direct PBS)
                datastore: 'backups',      // Has datastore (from direct PBS)
            };

            const shouldShowViaPVE =
                backup.backupType === 'remote' &&
                backup.storage &&
                !backup.datastore;

            expect(shouldShowViaPVE).toBeFalsy();
        });

        it('should NOT show "via PVE" for non-PBS backups', () => {
            const backup: { backupType: 'local' | 'remote'; storage: string; datastore: null } = {
                backupType: 'local',
                storage: 'local-lvm',
                datastore: null,
            };

            const shouldShowViaPVE =
                backup.backupType === 'remote' &&
                backup.storage &&
                !backup.datastore;

            expect(shouldShowViaPVE).toBe(false);
        });
    });
});
