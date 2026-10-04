// Mount the production SSO settings panel; the browser proof replaces only its API.
import { render } from 'solid-js/web';
import { SSOProvidersPanel } from '../src/components/Settings/SSOProvidersPanel';
import '../src/index.css';

render(() => (
  <main class="mx-auto max-w-5xl p-4">
    <SSOProvidersPanel />
  </main>
), document.getElementById('root')!);
