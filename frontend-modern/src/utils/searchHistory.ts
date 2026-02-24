import { logger } from '@/utils/logger';

const DEFAULT_MAX_HISTORY = 10;

type HistoryEntry = string;

function isBrowser(): boolean {
  return typeof window !== 'undefined' && typeof window.localStorage !== 'undefined';
}

function readHistory(key: string): HistoryEntry[] {
  if (!isBrowser()) {
    return [];
  }
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) {
      return [];
    }
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed.filter((value): value is string => typeof value === 'string');
  } catch (error) {
    logger.warn(`[searchHistory] Failed to parse history for key "${key}":`, error);
    return [];
  }
}

function writeHistory(key: string, history: HistoryEntry[]): void {
  if (!isBrowser()) {
    return;
  }
  try {
    window.localStorage.setItem(key, JSON.stringify(history));
  } catch (error) {
    logger.warn(`[searchHistory] Failed to persist history for key "${key}":`, error);
  }
}

export function getSearchHistory(key: string): HistoryEntry[] {
  return readHistory(key);
}

export function addSearchHistory(
  key: string,
  term: string,
  maxEntries: number = DEFAULT_MAX_HISTORY,
): HistoryEntry[] {
  const trimmed = term.trim();
  if (!trimmed) {
    return readHistory(key);
  }

  const history = readHistory(key);
  const lowerTerm = trimmed.toLowerCase();
  const filtered = history.filter((entry) => entry.toLowerCase() !== lowerTerm);
  const nextHistory = [trimmed, ...filtered].slice(0, Math.max(1, maxEntries));
  writeHistory(key, nextHistory);
  return nextHistory;
}

export function removeSearchHistory(key: string, term: string): HistoryEntry[] {
  const history = readHistory(key);
  const nextHistory = history.filter((entry) => entry !== term);
  writeHistory(key, nextHistory);
  return nextHistory;
}

export function clearSearchHistory(key: string): HistoryEntry[] {
  writeHistory(key, []);
  return [];
}

export function createSearchHistoryManager(key: string, options?: { maxEntries?: number }) {
  const maxEntries = options?.maxEntries ?? DEFAULT_MAX_HISTORY;

  const read = () => getSearchHistory(key);

  const add = (term: string) => addSearchHistory(key, term, maxEntries);
  const remove = (term: string) => removeSearchHistory(key, term);
  const clear = () => clearSearchHistory(key);

  return { read, add, remove, clear };
}
