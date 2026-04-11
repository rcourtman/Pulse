import { describe, expect, it } from 'vitest';
import appSource from '@/App.tsx?raw';
import storagePageRouteSource from '@/pages/Storage.tsx?raw';
import storageSurfaceSource from '@/components/Storage/Storage.tsx?raw';

describe('storage page route shell', () => {
  it('keeps App routing on a page shell instead of the storage surface component', () => {
    expect(appSource).toContain("const StoragePage = lazy(() => import('./pages/Storage'));");
    expect(appSource).not.toContain(
      "const StorageComponent = lazy(() => import('./components/Storage/Storage'));",
    );
    expect(storagePageRouteSource).toContain(
      "import StorageSurface from '@/components/Storage/Storage';",
    );
    expect(storagePageRouteSource).toContain('<StorageSurface />');
    expect(storagePageRouteSource).not.toContain('useStoragePageModel');
    expect(storageSurfaceSource).toContain("import { PageHeader } from '@/components/shared/PageHeader';");
    expect(storageSurfaceSource).toContain('<PageHeader');
    expect(storageSurfaceSource).toContain('title="Storage"');
    expect(storageSurfaceSource).toContain('useStoragePageModel');
  });
});
