import { describe, expect, it } from 'vitest';

import { APP_SHELL_ROUTE_PRELOAD_PATHS } from '../routePreload';
import { PATROL_PATH, buildAgentsPath, buildProxmoxPath } from '../resourceLinks';

describe('route preloading', () => {
  it('keeps Proxmox in the authenticated app-shell preload set', () => {
    expect(APP_SHELL_ROUTE_PRELOAD_PATHS).toContain(buildProxmoxPath());
  });

  it('keeps all cold top-level app shell targets in the shared preload set', () => {
    expect([...APP_SHELL_ROUTE_PRELOAD_PATHS]).toEqual([
      buildAgentsPath(),
      buildProxmoxPath(),
      PATROL_PATH,
      '/alerts',
      '/settings',
    ]);
  });
});
