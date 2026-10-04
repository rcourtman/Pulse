// Exercise the production Actions page with scripted, read-only action responses.
import { Route, Router } from '@solidjs/router';
import { render } from 'solid-js/web';
import Actions from '../src/pages/Actions';
import '../src/index.css';

render(
  () => (
    <Router>
      <Route path="*" component={Actions} />
    </Router>
  ),
  document.getElementById('root')!,
);
