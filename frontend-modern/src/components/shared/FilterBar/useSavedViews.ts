import { useLocation, useNavigate } from '@solidjs/router';
import { Accessor, createSignal, onMount } from 'solid-js';

export interface SavedView {
  id: string;
  name: string;
  query: string;
  savedAt: number;
}

const STORAGE_PREFIX = 'pulse:filterbar:saved-views:';

const storageKeyFor = (key: string): string => `${STORAGE_PREFIX}${key}`;

const isSavedView = (entry: unknown): entry is SavedView =>
  Boolean(entry) &&
  typeof entry === 'object' &&
  typeof (entry as SavedView).id === 'string' &&
  typeof (entry as SavedView).name === 'string' &&
  typeof (entry as SavedView).query === 'string' &&
  typeof (entry as SavedView).savedAt === 'number';

const readStored = (key: string): SavedView[] => {
  if (typeof window === 'undefined') return [];
  try {
    const raw = window.localStorage.getItem(storageKeyFor(key));
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(isSavedView);
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

export interface UseSavedViewsResult {
  views: Accessor<SavedView[]>;
  saveCurrent: (name: string) => SavedView | null;
  removeView: (id: string) => void;
  applyView: (view: SavedView) => void;
}

export function useSavedViews(key: string): UseSavedViewsResult {
  const navigate = useNavigate();
  const location = useLocation();
  const [views, setViews] = createSignal<SavedView[]>([]);

  onMount(() => {
    setViews(readStored(key));
  });

  const saveCurrent = (name: string): SavedView | null => {
    const trimmed = name.trim();
    if (trimmed === '') return null;
    const query =
      typeof window !== 'undefined' ? window.location.search.replace(/^\?/, '') : '';
    const view: SavedView = {
      id: generateId(),
      name: trimmed,
      query,
      savedAt: Date.now(),
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

  const applyView = (view: SavedView): void => {
    const path = `${location.pathname}${view.query ? `?${view.query}` : ''}`;
    navigate(path, { replace: false });
  };

  return { views, saveCurrent, removeView, applyView };
}
