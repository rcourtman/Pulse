// Shared app runtime for browser fixtures. Drawers read active alerts and the
// theme from the app runtime contexts and some components use router links, so
// a fixture that mounts them without these providers throws before rendering.
// The WebSocket context is an explicit offline one: no live socket, no alerts.
import type { Component } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';

import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';

export const renderInAppRuntime = (Fixture: Component, root = document.getElementById('root')) =>
  render(
    () => (
      <WebSocketContext.Provider
        value={{ state: { pmg: [] }, activeAlerts: {}, connected: () => false } as never}
      >
        <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
          <Router>
            <Route path="/*" component={Fixture} />
          </Router>
        </DarkModeContext.Provider>
      </WebSocketContext.Provider>
    ),
    root as HTMLElement,
  );
