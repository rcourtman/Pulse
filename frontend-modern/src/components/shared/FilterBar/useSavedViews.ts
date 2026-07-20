import { useLocation, useNavigate } from '@solidjs/router';
import { Accessor, createSignal, onMount } from 'solid-js';

export interface SavedView {
  id: string;
  name: string;
  query: string;
  savedAt: number;
  version: number;
  isDefault?: boolean;
}

// Bump when the on-wire shape of a saved view's `query` changes incompatibly
// (e.g. a FilterDef renames its URL key or changes encoding). Reads tolerate
// missing version by treating entries as v1.
const CURRENT_VERSION = 1;

const STORAGE_PREFIX = 'pulse:filterbar:saved-views:';

const storageKeyFor = (key: string): string => `${STORAGE_PREFIX}${key}`;

const isSavedView = (entry: unknown): entry is SavedView =>
  Boolean(entry) &&
  typeof entry === 'object' &&
  typeof (entry as SavedView).id === 'string' &&
  typeof (entry as SavedView).name === 'string' &&
  typeof (entry as SavedView).query === 'string' &&
  typeof (entry as SavedView).savedAt === 'number';

const normalize = (entry: SavedView): SavedView => ({
  ...entry,
  version: typeof entry.version === 'number' ? entry.version : 1,
  isDefault: entry.isDefault === true,
});

const readStored = (key: string): SavedView[] => {
  if (typeof window === 'undefined') return [];
  try {
    const raw = window.localStorage.getItem(storageKeyFor(key));
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(isSavedView).map(normalize);
  } catch {
    return [];
  }
};

const writeStored = (key: string, views: SavedView[]): void => {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(storageKeyFor(key), JSON.stringify(views));
  } catch {
    // localStorage full / blocked: ignore so the UI still renders
  }
};

const generateId = (): string => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `view-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
};

export interface ApplyViewOptions {
  replace?: boolean;
}

export interface UseSavedViewsResult {
  views: Accessor<SavedView[]>;
  saveCurrent: (name: string) => SavedView | null;
  removeView: (id: string) => void;
  applyView: (view: SavedView, options?: ApplyViewOptions) => void;
  setDefault: (id: string) => void;
  clearDefault: () => void;
}

export function useSavedViews(key: string): UseSavedViewsResult {
  const navigate = useNavigate();
  const location = useLocation();
  const [views, setViews] = createSignal<SavedView[]>([]);

  const applyView = (view: SavedView, options?: ApplyViewOptions): void => {
    const path = `${location.pathname}${view.query ? `?${view.query}` : ''}`;
    navigate(path, { replace: options?.replace ?? false });
  };

  onMount(() => {
    const stored = readStored(key);
    setViews(stored);
    // Auto-apply default view on a fresh landing only. If the URL already
    // carries a query the user (or a deep link) chose it explicitly and we
    // leave it alone. Replace the history entry so the back button doesn't
    // bounce against an empty-query state that would just re-fire the
    // default and trap navigation.
    if (typeof window !== 'undefined' && window.location.search === '') {
      const fallback = stored.find((view) => view.isDefault === true);
      if (fallback) {
        applyView(fallback, { replace: true });
      }
    }
  });

  const saveCurrent = (name: string): SavedView | null => {
    const trimmed = name.trim();
    if (trimmed === '') return null;
    const query = typeof window !== 'undefined' ? window.location.search.replace(/^\?/, '') : '';
    const view: SavedView = {
      id: generateId(),
      name: trimmed,
      query,
      savedAt: Date.now(),
      version: CURRENT_VERSION,
    };
    const next = [...views(), view];
    setViews(next);
    writeStored(key, next);
    return view;
  };

  const removeView = (id: string): void => {
    const next = views().filter((entry) => entry.id !== id);
    setViews(next);
    writeStored(key, next);
  };

  const setDefault = (id: string): void => {
    const next = views().map((view) => ({
      ...view,
      isDefault: view.id === id,
    }));
    setViews(next);
    writeStored(key, next);
  };

  const clearDefault = (): void => {
    const next = views().map((view) => (view.isDefault ? { ...view, isDefault: false } : view));
    setViews(next);
    writeStored(key, next);
  };

  return { views, saveCurrent, removeView, applyView, setDefault, clearDefault };
}
