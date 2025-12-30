/**
 * Layout utilities for managing full-width mode preference
 */
import { createSignal } from 'solid-js';
import { STORAGE_KEYS } from './localStorage';

export type LayoutMode = 'default' | 'full-width';

/**
 * Creates a reactive store for layout mode preference
 */
function createLayoutStore() {
    const stored = localStorage.getItem(STORAGE_KEYS.FULL_WIDTH_MODE);
    const initialMode: LayoutMode = stored === 'full-width' ? 'full-width' : 'default';

    const [mode, setModeInternal] = createSignal<LayoutMode>(initialMode);

    const setMode = (newMode: LayoutMode) => {
        localStorage.setItem(STORAGE_KEYS.FULL_WIDTH_MODE, newMode);
        setModeInternal(newMode);
    };

    const toggle = () => {
        const newMode = mode() === 'default' ? 'full-width' : 'default';
        setMode(newMode);
    };

    const isFullWidth = () => mode() === 'full-width';

    return {
        mode,
        setMode,
        toggle,
        isFullWidth,
    };
}

export const layoutStore = createLayoutStore();
