// Mount the production Patrol surface under a local Router; the browser script
// supplies bounded API responses for the off/setup and attention states.
import { Route, Router } from '@solidjs/router';
import { render } from 'solid-js/web';
import { PatrolIntelligenceSurface } from '../src/features/patrol/PatrolIntelligenceSurface';
import '../src/index.css';

render(() => (
  <Router>
    <Route path="*" component={() => (
      <main class="mx-auto max-w-6xl p-4">
        <PatrolIntelligenceSurface />
      </main>
    )} />
  </Router>
), document.getElementById('root')!);
