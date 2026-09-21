// Browser fixture for issue #2119: the Alerts overview alert-card footer.
//
// It reproduces the production footer from AlertOverviewAlertCard.tsx: a flex
// row (items-center) whose first child is the "Started" line classed by
// getAlertOverviewStartedAtClass(), followed by the delivery-status span. The
// reporter's screenshot underlines the Started timestamp and the status text at
// different heights, so the fixture exists to measure whether the two runs share
// a baseline. Synthetic props only; no backend, API or WebSocket path.
import { render } from 'solid-js/web';

import { getAlertOverviewStartedAtClass } from '../src/utils/alertOverviewPresentation';
import '../src/index.css';

function AlertCardFooter() {
  return (
    <div
      id="alert-card"
      style={{
        width: '960px',
        margin: '24px',
        padding: '16px',
        border: '1px solid #f59e0b',
        'border-radius': '8px',
        background: '#111827',
        color: '#9ca3af',
      }}
    >
      <p style={{ margin: '0', 'font-size': '14px', color: '#e5e7eb' }}>
        Alert notifications are not reaching their destinations.
      </p>
      <div id="alert-card-footer" class="flex flex-wrap items-center gap-x-3 gap-y-0.5 mt-1">
        <p class={getAlertOverviewStartedAtClass()} data-testid="started">
          Started: 9/18/2026, 12:15:58 PM
        </p>
        <span class="text-xs text-muted" data-testid="status">
          Notified 12:15 PM
        </span>
      </div>
    </div>
  );
}

render(() => <AlertCardFooter />, document.getElementById('root')!);
