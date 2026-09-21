// Browser fixture: the production Docs page under the production router.
//
// It exists to verify #2067's shipped-documentation navigation in a real
// browser: GitHub-compatible heading IDs, direct/reload fragment focus and
// scrolling after the asynchronous Markdown fetch, malformed-fragment safety
// and keyboard activation of intra-document links. It serves the real
// `frontend-modern/public/docs/API.md` asset from the Vite dev server, so no
// backend, API or WebSocket path is used.
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';

import Docs from '../src/pages/Docs';
import '../src/index.css';

const scenario = new URLSearchParams(window.location.search).get('scenario') ?? 'plain';

const targets: Record<string, string> = {
  plain: '/docs/API',
  direct: '/docs/API#resource-maintenance-and-operator-state',
  reload: '/docs/API#resource-maintenance-and-operator-state',
  malformed: '/docs/API#%invalid',
  missing: '/docs/API#does-not-exist',
};

window.history.replaceState({}, '', targets[scenario] ?? targets.plain);

render(
  () => (
    <Router>
      <Route path="/docs" component={Docs} />
      <Route path="/docs/*docPath" component={Docs} />
      <Route path="*" component={() => <p>not found</p>} />
    </Router>
  ),
  document.getElementById('root')!,
);
