import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import Docs from '../src/pages/Docs';
import '../src/index.css';

// Use the real fetch/renderer/router. Only the surrounding application shell
// is a fixture; no host, agent, credential or recovery action is executed.
render(() => (
  <Router root={props => (
    <main class="min-h-screen bg-background p-4 text-base-content sm:p-6">
      {props.children}
    </main>
  )}>
    <Route path="/docs/*docPath" component={Docs} />
  </Router>
), document.getElementById('root')!);
