// Production Diagnostics, its API reader, exporter and Docs viewer.
// The browser script supplies only synthetic diagnostic responses; no target
// API or guest-agent check is performed by this fixture.
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import DiagnosticsPanel from '../src/components/Settings/DiagnosticsPanel';
import { ToastContainer } from '../src/components/Toast/Toast';
import Docs from '../src/pages/Docs';
import '../src/index.css';

function Diagnostics() {
  return (
    <main class="mx-auto max-w-5xl p-3 sm:p-6 text-base-content">
      <h1 class="mb-4 text-xl font-semibold">Settings — Diagnostics</h1>
      <DiagnosticsPanel />
      <ToastContainer />
    </main>
  );
}

render(
  () => (
    <Router>
      <Route path="/docs/*docPath" component={Docs} />
      <Route path="/*" component={Diagnostics} />
    </Router>
  ),
  document.getElementById('root')!,
);
