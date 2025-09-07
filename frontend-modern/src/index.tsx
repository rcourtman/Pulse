/* @refresh reload */
import { render } from 'solid-js/web';
import './index.css';
import './styles/animations.css';
import App from './App';
// import App from './Test';
import { logger } from './utils/logger';

const root = document.getElementById('root');

if (import.meta.env.DEV && !(root instanceof HTMLElement)) {
  throw new Error(
    'Root element not found. Did you forget to add it to your index.html? Or maybe the id attribute got misspelled?',
  );
}

// Initialize app with logging
console.log('[Index] Starting Pulse app...');
logger.info('Pulse monitoring dashboard starting');


if (root) {
  console.log('[Index] Root element found, rendering App...');
  try {
    render(() => <App />, root);
    console.log('[Index] Render call completed');
  } catch (error) {
    console.error('[Index] Render error:', error);
    // Show error on page
    root.innerHTML = `<div style="color: red; padding: 20px;">
      <h1>Error Loading App</h1>
      <pre>${error}</pre>
    </div>`;
  }
} else {
  console.error('[Index] Root element not found!');
}