import { describe, expect, it } from 'vitest';

import { APP_SHELL_ROUTE_PRELOAD_PATHS } from '../routePreload';
import {
  PATROL_PATH,
  buildProxmoxPath,
  buildRecoveryPath,
  buildStoragePath,
  buildWorkloadsPath,
} from '../resourceLinks';

describe('route preloading', () => {
  it('keeps Workloads in the authenticated app-shell preload set', () => {
    expect(APP_SHELL_ROUTE_PRELOAD_PATHS).toContain(buildWorkloadsPath());
  });

  it('keeps all cold top-level app shell targets in the shared preload set', () => {
    expect([...APP_SHELL_ROUTE_PRELOAD_PATHS]).toEqual([
      buildProxmoxPath(),
      buildWorkloadsPath(),
      buildRecoveryPath(),
      PATROL_PATH,
      '/alerts',
      buildStoragePath(),
      '/settings',
    ]);
  });
});
