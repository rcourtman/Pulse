import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import Docs from '../src/pages/Docs';
import { AISettings } from '../src/components/Settings/AISettings';
import '../src/index.css';

// Only the shell and server data are synthetic. Exercise the actual Docs
// consumer and Provider & Models, including its state, API parser and save.
render(() => (
  <Router root={props => (
    <main class="mx-auto min-h-screen max-w-5xl bg-background p-4 text-base-content sm:p-6">
      {props.children}
    </main>
  )}>
    <Route path="/docs/*docPath" component={Docs} />
    <Route path="/settings" component={AISettings} />
  </Router>
), document.getElementById('root')!);
