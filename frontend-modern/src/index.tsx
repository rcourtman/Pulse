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

logger.info('Pulse monitoring workspace starting');

if (root) {
  logger.debug('[Index] Root element found, rendering App...');
  try {
    render(() => <App />, root);
    logger.debug('[Index] Render call completed');
  } catch (error) {
    logger.error('[Index] Render error', error);
    // Show error on page. Built with DOM nodes and classes: the production CSP
    // refuses inline style attributes, and the error text is not markup.
    const fallback = document.createElement('div');
    fallback.className = 'p-5 text-red-600';
    const heading = document.createElement('h1');
    heading.textContent = 'Error Loading App';
    const detail = document.createElement('pre');
    detail.textContent = String(error);
    fallback.append(heading, detail);
    root.replaceChildren(fallback);
  }
} else {
  logger.error('[Index] Root element not found', new Error('root element missing'));
}
