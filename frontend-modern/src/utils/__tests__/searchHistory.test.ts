import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
    getSearchHistory,
    addSearchHistory,
    clearSearchHistory,
    removeSearchHistory
} from '../searchHistory';

describe('searchHistory utils', () => {
    const mockStorage: Record<string, string> = {};
    const TEST_KEY = 'test_history';

    beforeEach(() => {
        // Mock localStorage
        Object.defineProperty(window, 'localStorage', {
            value: {
                getItem: vi.fn((key: string) => mockStorage[key] || null),
                setItem: vi.fn((key: string, value: string) => { mockStorage[key] = value; }),
                removeItem: vi.fn((key: string) => { delete mockStorage[key]; }),
                clear: vi.fn(() => { for (const key in mockStorage) delete mockStorage[key]; }),
            },
            writable: true
        });
        // Clear mock storage manually since we're using a local object
        for (const key in mockStorage) delete mockStorage[key];
    });

    it('getSearchHistory returns empty array initially', () => {
        expect(getSearchHistory(TEST_KEY)).toEqual([]);
    });

    it('addSearchHistory adds item to history', () => {
        addSearchHistory(TEST_KEY, 'test query');
        expect(getSearchHistory(TEST_KEY)).toEqual(['test query']);
    });

    it('addSearchHistory moves existing item to front', () => {
        addSearchHistory(TEST_KEY, 'query 1');
        addSearchHistory(TEST_KEY, 'query 2');
        addSearchHistory(TEST_KEY, 'query 1');
        expect(getSearchHistory(TEST_KEY)).toEqual(['query 1', 'query 2']);
    });

    it('addSearchHistory limits history size', () => {
        for (let i = 0; i < 20; i++) {
            addSearchHistory(TEST_KEY, `query ${i}`, 10);
        }
        const history = getSearchHistory(TEST_KEY);
        expect(history.length).toBe(10);
        expect(history[0]).toBe('query 19');
    });

    it('removeSearchHistory removes specific item', () => {
        addSearchHistory(TEST_KEY, 'query 1');
        addSearchHistory(TEST_KEY, 'query 2');
        removeSearchHistory(TEST_KEY, 'query 1');
        expect(getSearchHistory(TEST_KEY)).toEqual(['query 2']);
    });

    it('clearSearchHistory clears everything', () => {
        addSearchHistory(TEST_KEY, 'query 1');
        clearSearchHistory(TEST_KEY);
        expect(getSearchHistory(TEST_KEY)).toEqual([]);
    });
});
