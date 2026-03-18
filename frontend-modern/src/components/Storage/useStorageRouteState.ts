import { Accessor, createEffect, onCleanup, untrack } from 'solid-js';
import { STORAGE_QUERY_PARAMS, parseStorageLinkSearch } from '@/routing/resourceLinks';
import { areSearchParamsEquivalent } from '@/utils/searchParams';

export type StorageManagedQueryKey =
  | 'tab'
  | 'group'
  | 'source'
  | 'status'
  | 'node'
  | 'query'
  | 'sort'
  | 'order';

export type StoragePathOptions = Partial<Record<StorageManagedQueryKey, string | null>>;

export type ParsedStorageSearch = ReturnType<typeof parseStorageLinkSearch>;

export type StorageRouteStateField<T = any> = {
  get: Accessor<T>;
  set: (value: T) => unknown;
  read: (parsed: ParsedStorageSearch) => T;
  write?: (value: T) => string | null;
};

export type StorageRouteStateFields = Partial<
  Record<StorageManagedQueryKey, StorageRouteStateField>
>;

type StorageRouteStateConfig = {
  location: {
    pathname: string;
    search: string;
  };
  navigate: (path: string, options: { replace: true }) => void;
  buildPath: (options: StoragePathOptions) => string;
  fields: StorageRouteStateFields;
  isReadEnabled?: () => boolean;
  isWriteEnabled?: () => boolean;
  useCurrentPathForNavigation?: boolean;
};

const MANAGED_QUERY_KEYS: StorageManagedQueryKey[] = [
  'tab',
  'group',
  'source',
  'status',
  'node',
  'query',
  'sort',
  'order',
];

const isEnabled = (predicate?: () => boolean) => (predicate ? predicate() : true);

export const useStorageRouteState = (config: StorageRouteStateConfig): void => {
  let pendingNavigateHandle: number | null = null;
  let pendingNavigatePath: string | null = null;

  const scheduleNavigate = (nextPath: string) => {
    pendingNavigatePath = nextPath;
    if (pendingNavigateHandle !== null) return;

    pendingNavigateHandle = window.setTimeout(() => {
      pendingNavigateHandle = null;
      const target = pendingNavigatePath;
      pendingNavigatePath = null;
      if (!target) return;
      const currentPath = `${untrack(() => config.location.pathname)}${untrack(() => config.location.search) || ''}`;
      if (currentPath === target) return;
      config.navigate(target, { replace: true });
    }, 0);
  };

  createEffect(() => {
    if (!isEnabled(config.isReadEnabled)) return;

    const parsed = parseStorageLinkSearch(config.location.search);

    for (const key of MANAGED_QUERY_KEYS) {
      const field = config.fields[key];
      if (!field) continue;
      const nextValue = field.read(parsed);
      if (nextValue !== untrack(field.get)) {
        field.set(nextValue);
      }
    }
  });

  createEffect(() => {
    if (!isEnabled(config.isWriteEnabled)) return;

    const options: StoragePathOptions = {};
    for (const key of MANAGED_QUERY_KEYS) {
      const field = config.fields[key];
      if (!field?.write) continue;
      options[key] = field.write(field.get());
    }

    const managedPath = config.buildPath(options);
    const [, managedSearch = ''] = managedPath.split('?');
    const managedParams = new URLSearchParams(managedSearch);
    const currentParams = new URLSearchParams(config.location.search);
    const nextParams = new URLSearchParams(config.location.search);

    MANAGED_QUERY_KEYS.forEach((key) => {
      nextParams.delete(STORAGE_QUERY_PARAMS[key]);
    });
    managedParams.forEach((value, key) => {
      nextParams.set(key, value);
    });

    if (areSearchParamsEquivalent(currentParams, nextParams)) {
      return;
    }

    const nextSearch = nextParams.toString();
    const basePath = config.useCurrentPathForNavigation
      ? config.location.pathname
      : managedPath.split('?')[0];
    const nextPath = nextSearch ? `${basePath}?${nextSearch}` : basePath;
    scheduleNavigate(nextPath);
  });

  onCleanup(() => {
    if (pendingNavigateHandle !== null) {
      window.clearTimeout(pendingNavigateHandle);
      pendingNavigateHandle = null;
      pendingNavigatePath = null;
    }
  });
};
