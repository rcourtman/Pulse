/**
 * Layout utilities for managing full-width mode preference
 */
import { createSignal } from 'solid-js';
import { STORAGE_KEYS } from './localStorage';
import { SettingsAPI } from '@/api/settings';
import { logger } from './logger';

export type LayoutMode = 'default' | 'full-width';

/**
 * Creates a reactive store for layout mode preference
 * Syncs with both localStorage (for immediate access) and server (for persistence across updates)
 */
function createLayoutStore() {
    const stored = localStorage.getItem(STORAGE_KEYS.FULL_WIDTH_MODE);
    const initialMode: LayoutMode = stored === 'full-width' ? 'full-width' : 'default';

    const [mode, setModeInternal] = createSignal<LayoutMode>(initialMode);
    const [hasLoadedFromServer, setHasLoadedFromServer] = createSignal(false);

    const setMode = (newMode: LayoutMode, syncToServer = true) => {
        localStorage.setItem(STORAGE_KEYS.FULL_WIDTH_MODE, newMode);
        setModeInternal(newMode);

        // Sync to server for persistence across updates
        if (syncToServer) {
            SettingsAPI.updateSystemSettings({ fullWidthMode: newMode === 'full-width' })
                .then(() => logger.debug('Full-width mode synced to server', { mode: newMode }))
                .catch((error) => logger.warn('Failed to sync full-width mode to server', error));
        }
    };

    const toggle = () => {
        const newMode = mode() === 'default' ? 'full-width' : 'default';
        setMode(newMode);
    };

    const isFullWidth = () => mode() === 'full-width';

    /**
     * Load full-width preference from server (called after auth)
     * Always uses server preference after auth to keep localStorage in sync
     * (server is the source of truth for cross-device/browser persistence).
     */
    const loadFromServer = async () => {
        if (hasLoadedFromServer()) {
            return; // Already loaded this session
        }

        try {
            const settings = await SettingsAPI.getSystemSettings();
            if (settings.fullWidthMode !== undefined) {
                const serverMode: LayoutMode = settings.fullWidthMode ? 'full-width' : 'default';
                localStorage.setItem(STORAGE_KEYS.FULL_WIDTH_MODE, serverMode);
                setModeInternal(serverMode);
                logger.debug('Loaded full-width mode from server', { mode: serverMode });
            }
            setHasLoadedFromServer(true);
        } catch (error) {
            logger.warn('Failed to load full-width mode from server', error);
        }
    };

    return {
        mode,
        setMode,
        toggle,
        isFullWidth,
        loadFromServer,
    };
}

export const layoutStore = createLayoutStore();
