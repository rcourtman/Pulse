// The production installation owner, command producer and consuming panel.
// The browser driver supplies synthetic GETs; no token mint or install occurs.
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { InfrastructureInstallerSection } from '../src/components/Settings/InfrastructureInstallerSection';
import { InfrastructureOperationsStateProvider } from '../src/components/Settings/useInfrastructureOperationsState';
import '../src/index.css';

render(
  () => (
    <Router>
      <Route
        path="/*"
        component={() => (
          <main class="mx-auto max-w-4xl p-3 text-base-content">
            <InfrastructureOperationsStateProvider embedded>
              <InfrastructureInstallerSection />
            </InfrastructureOperationsStateProvider>
          </main>
        )}
      />
    </Router>
  ),
  document.getElementById('root')!,
);
