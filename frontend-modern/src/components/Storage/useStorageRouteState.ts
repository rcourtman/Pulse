import { Accessor, createEffect, untrack } from 'solid-js';
import { STORAGE_QUERY_PARAMS, parseStorageLinkSearch } from '@/routing/resourceLinks';

type StorageManagedQueryKey =
  | 'tab'
  | 'group'
  | 'source'
  | 'status'
  | 'node'
  | 'query'
  | 'sort'
  | 'order';

type StoragePathOptions = Partial<Record<StorageManagedQueryKey, string | null>>;

type ParsedStorageSearch = ReturnType<typeof parseStorageLinkSearch>;

type StorageRouteStateField<T = any> = {
  get: Accessor<T>;
  set: (value: T) => unknown;
  read: (parsed: ParsedStorageSearch) => T;
  write?: (value: T) => string | null;
};

type StorageRouteStateConfig = {
  location: {
    pathname: string;
    search: string;
  };
  navigate: (path: string, options: { replace: true }) => void;
  buildPath: (options: StoragePathOptions) => string;
  fields: Partial<Record<StorageManagedQueryKey, StorageRouteStateField>>;
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
    const [managedBasePath, managedSearch = ''] = managedPath.split('?');
    const managedParams = new URLSearchParams(managedSearch);
    const params = new URLSearchParams(config.location.search);

    Object.values(STORAGE_QUERY_PARAMS).forEach((queryKey) => {
      params.delete(queryKey);
    });
    managedParams.forEach((value, key) => {
      params.set(key, value);
    });

    const nextSearch = params.toString();
    const basePath = config.useCurrentPathForNavigation
      ? config.location.pathname
      : managedBasePath;
    const nextPath = nextSearch ? `${basePath}?${nextSearch}` : basePath;
    const currentPath = `${config.location.pathname}${config.location.search || ''}`;
    if (nextPath !== currentPath) {
      config.navigate(nextPath, { replace: true });
    }
  });
};
