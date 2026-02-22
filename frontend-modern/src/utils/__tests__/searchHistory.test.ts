import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
    getSearchHistory,
    addSearchHistory,
    clearSearchHistory,
    removeSearchHistory,
    createSearchHistoryManager
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

    it('addSearchHistory trims whitespace', () => {
        addSearchHistory(TEST_KEY, '  test query  ');
        expect(getSearchHistory(TEST_KEY)).toEqual(['test query']);
    });

    it('addSearchHistory ignores empty strings after trim', () => {
        addSearchHistory(TEST_KEY, '  ');
        addSearchHistory(TEST_KEY, '');
        expect(getSearchHistory(TEST_KEY)).toEqual([]);
    });

    it('addSearchHistory is case-insensitive for duplicate detection', () => {
        addSearchHistory(TEST_KEY, 'Test Query');
        addSearchHistory(TEST_KEY, 'test query');
        expect(getSearchHistory(TEST_KEY)).toEqual(['test query']);
    });

    it('removeSearchHistory returns empty array when item not found', () => {
        removeSearchHistory(TEST_KEY, 'nonexistent');
        expect(getSearchHistory(TEST_KEY)).toEqual([]);
    });

    it('handles corrupted JSON in localStorage gracefully', () => {
        mockStorage[TEST_KEY] = 'not valid json';
        expect(getSearchHistory(TEST_KEY)).toEqual([]);
    });

    it('handles non-array JSON in localStorage gracefully', () => {
        mockStorage[TEST_KEY] = JSON.stringify({ not: 'array' });
        expect(getSearchHistory(TEST_KEY)).toEqual([]);
    });

    it('handles array with non-string values gracefully', () => {
        mockStorage[TEST_KEY] = JSON.stringify(['valid', 123, null]);
        expect(getSearchHistory(TEST_KEY)).toEqual(['valid']);
    });
});

describe('createSearchHistoryManager', () => {
    const TEST_KEY = 'manager_test';
    const mockStorage: Record<string, string> = {};

    beforeEach(() => {
        Object.defineProperty(window, 'localStorage', {
            value: {
                getItem: vi.fn((key: string) => mockStorage[key] || null),
                setItem: vi.fn((key: string, value: string) => { mockStorage[key] = value; }),
                removeItem: vi.fn((key: string) => { delete mockStorage[key]; }),
                clear: vi.fn(() => { for (const key in mockStorage) delete mockStorage[key]; }),
            },
            writable: true
        });
        for (const key in mockStorage) delete mockStorage[key];
    });

    it('creates manager with read, add, remove, clear methods', () => {
        const manager = createSearchHistoryManager(TEST_KEY);
        expect(typeof manager.read).toBe('function');
        expect(typeof manager.add).toBe('function');
        expect(typeof manager.remove).toBe('function');
        expect(typeof manager.clear).toBe('function');
    });

    it('uses custom maxEntries from options', () => {
        const manager = createSearchHistoryManager(TEST_KEY, { maxEntries: 5 });
        manager.add('query 1');
        manager.add('query 2');
        manager.add('query 3');
        manager.add('query 4');
        manager.add('query 5');
        manager.add('query 6');
        expect(manager.read().length).toBe(5);
    });

    it('read returns current history', () => {
        const manager = createSearchHistoryManager(TEST_KEY);
        manager.add('test query');
        expect(manager.read()).toEqual(['test query']);
    });

    it('add adds to history', () => {
        const manager = createSearchHistoryManager(TEST_KEY);
        manager.add('new query');
        expect(manager.read()).toEqual(['new query']);
    });

    it('remove removes specific entry', () => {
        const manager = createSearchHistoryManager(TEST_KEY);
        manager.add('query 1');
        manager.add('query 2');
        manager.remove('query 1');
        expect(manager.read()).toEqual(['query 2']);
    });

    it('clear removes all entries', () => {
        const manager = createSearchHistoryManager(TEST_KEY);
        manager.add('query 1');
        manager.add('query 2');
        manager.clear();
        expect(manager.read()).toEqual([]);
    });
});
