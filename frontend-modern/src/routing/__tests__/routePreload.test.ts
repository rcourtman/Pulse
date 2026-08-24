import { describe, expect, it } from 'vitest';

import { APP_SHELL_ROUTE_PRELOAD_PATHS } from '../routePreload';
import { ACTIONS_PATH } from '../resourceLinks';

describe('route preloading', () => {
  it('preloads only the lightweight global Actions review route', () => {
    expect([...APP_SHELL_ROUTE_PRELOAD_PATHS]).toEqual([ACTIONS_PATH]);
  });
});
