/* @refresh reload */
import { render } from 'solid-js/web';
import './index.css';
import './styles/animations.css';
import App from './App';
import { logger } from './utils/logger';

const root = document.getElementById('root');

if (import.meta.env.DEV && !(root instanceof HTMLElement)) {
  throw new Error(
    'Root element not found. Did you forget to add it to your index.html? Or maybe the id attribute got misspelled?',
  );
}

// Initialize app with logging
logger.info('Pulse monitoring dashboard starting');


if (root) {
  render(() => <App />, root);
}