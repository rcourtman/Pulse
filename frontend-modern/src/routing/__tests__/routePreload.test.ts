import { describe, expect, it } from 'vitest';

import { APP_SHELL_ROUTE_PRELOAD_PATHS } from '../routePreload';
import { PATROL_PATH, buildProxmoxPath, buildStandalonePath } from '../resourceLinks';

describe('route preloading', () => {
  it('keeps Proxmox in the authenticated app-shell preload set', () => {
    expect(APP_SHELL_ROUTE_PRELOAD_PATHS).toContain(buildProxmoxPath());
  });

  it('keeps the eager authenticated-shell preload set bounded', () => {
    expect([...APP_SHELL_ROUTE_PRELOAD_PATHS]).toEqual([
      buildProxmoxPath(),
      buildStandalonePath(),
      PATROL_PATH,
      '/alerts',
      '/settings',
    ]);
  });
});
