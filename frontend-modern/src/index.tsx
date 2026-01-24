/* @refresh reload */
import { render } from 'solid-js/web';
import './index.css';
import App from './App';
import { logger } from './utils/logger';

const root = document.getElementById('root');

if (import.meta.env.DEV && !(root instanceof HTMLElement)) {
  throw new Error(
    'Root element not found. Did you forget to add it to your index.html? Or maybe the id attribute got misspelled?',
  );
}

logger.info('Pulse monitoring dashboard starting');

if (root) {
  logger.debug('[Index] Root element found, rendering App...');
  try {
    render(() => <App />, root);
    logger.debug('[Index] Render call completed');
  } catch (error) {
    logger.error('[Index] Render error', error);
    // Show error on page
    root.innerHTML = `<div style="color: red; padding: 20px;">
      <h1>Error Loading App</h1>
      <pre>${error}</pre>
    </div>`;
  }
} else {
  logger.error('[Index] Root element not found', new Error('root element missing'));
}
