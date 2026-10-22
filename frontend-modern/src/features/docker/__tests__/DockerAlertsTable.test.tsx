import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import { Route, Router } from '@solidjs/router';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { DockerAlertsTable } from '../DockerAlertsTable';
import { buildDockerIncidentRows } from '../dockerPageModel';

// Record drawers link to the object they are about, which needs a router.
const renderInRouter = (component: () => JSX.Element) =>
  render(() => (
    <Router>
      <Route path="/*" component={component} />
    </Router>
  ));

const makeResource = ({
  id,
  type = 'agent',
  ...overrides
}: Partial<Resource> & Pick<Resource, 'id'>): Resource => ({
  id,
  name: id,
  displayName: id,
  platformId: 'lab',
  platformType: 'docker',
  sourceType: 'agent',
  sources: ['docker'],
  status: 'online',
  type,
  lastSeen: 1_700_000_000_000,
  ...overrides,
});

afterEach(() => {
  cleanup();
  window.history.replaceState(null, '', '/');
});

describe('DockerAlertsTable', () => {
  it('links an alert drawer to its container, and not to a tab that cannot focus it', async () => {
    const incidents = buildDockerIncidentRows([
      makeResource({
        id: 'ctr-payments',
        type: 'app-container',
        name: 'payments-worker',
        displayName: 'payments-worker',
        docker: { hostname: 'edge-01' },
        incidents: [
          { code: 'docker_container_restarting', severity: 'warning', summary: 'Restart loop' },
        ],
      }),
      makeResource({
        id: 'img-stale',
        type: 'docker-image',
        name: 'ghcr.io/pulse-demo/api:2026.04',
        displayName: 'ghcr.io/pulse-demo/api:2026.04',
        incidents: [
          { code: 'docker_image_update', severity: 'info', summary: 'Image update available' },
        ],
      }),
    ]);
    renderInRouter(() => (
      <DockerAlertsTable
        incidents={incidents}
        emptyIcon={<span />}
        emptyTitle="No alerts"
        emptyDescription="No alerts"
        showToolbar={false}
      />
    ));

    await fireEvent.click(
      screen.getByRole('button', { name: 'Expand details for payments-worker' }),
    );
    const link = within(screen.getByTestId('docker-alert-detail')).getByRole('link', {
      name: 'Open payments-worker',
    });
    expect(link).toHaveAttribute('href', '/docker/overview?q=payments-worker');
    // The drawer contains its clicks; the link still navigates in the app.
    await fireEvent.click(link);
    await waitFor(() => expect(window.location.pathname).toBe('/docker/overview'));
    expect(window.location.search).toBe('?q=payments-worker');

    await fireEvent.click(
      screen.getByRole('button', { name: 'Expand details for ghcr.io/pulse-demo/api:2026.04' }),
    );
    expect(screen.getAllByTestId('docker-alert-detail')).toHaveLength(1);
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });

  it('renders incident rows critical → warning → info with severity-coloured dots', () => {
    const incidents = buildDockerIncidentRows([
      makeResource({
        id: 'host-edge',
        type: 'agent',
        name: 'edge-01',
        docker: { hostname: 'edge-01', runtime: 'docker' },
        incidents: [
          { code: 'docker_host_down', severity: 'critical', summary: 'Engine unreachable' },
        ],
      }),
      makeResource({
        id: 'ctr-payments',
        type: 'app-container',
        name: 'payments-worker',
        docker: { hostname: 'edge-01' },
        incidents: [
          {
            code: 'docker_container_restarting',
            severity: 'warning',
            summary: 'Restart loop',
          },
        ],
      }),
      makeResource({
        id: 'img-stale',
        type: 'docker-image',
        name: 'ghcr.io/pulse-demo/api:2026.04',
        incidents: [
          { code: 'docker_image_update', severity: 'info', summary: 'Image update available' },
        ],
      }),
    ]);

    renderInRouter(() => (
      <DockerAlertsTable
        incidents={incidents}
        emptyIcon={<span />}
        emptyTitle="No Docker alerts"
        emptyDescription="No Docker alerts"
        showToolbar={false}
      />
    ));

    const rows = Array.from(document.querySelectorAll('[data-docker-alert-row]')).map((row) =>
      row.getAttribute('data-docker-alert-row'),
    );
    expect(rows[0]).toContain('host-edge:incident:docker_host_down');
    expect(rows[1]).toContain('ctr-payments:incident:docker_container_restarting');
    expect(rows[2]).toContain('img-stale:incident:docker_image_update');

    expect(screen.getAllByTitle('Critical')[0]).toHaveClass('bg-red-500');
    expect(screen.getAllByTitle('Warning')[0]).toHaveClass('bg-amber-500');
    expect(screen.getAllByTitle('Info')[0]).toHaveClass('bg-slate-400');

    expect(screen.getByText('Engine unreachable')).toBeInTheDocument();
    expect(screen.getByText('Restart loop')).toBeInTheDocument();
    expect(screen.getByText('Image update available')).toBeInTheDocument();
  });

  it('renders the empty-state fallback when given no incidents', () => {
    renderInRouter(() => (
      <DockerAlertsTable
        incidents={[]}
        emptyIcon={<span data-testid="empty-icon" />}
        emptyTitle="No Docker alerts"
        emptyDescription="Active alerts appear here."
        showToolbar={false}
      />
    ));
    expect(screen.getByText('No Docker alerts')).toBeInTheDocument();
    expect(document.querySelector('[data-docker-alert-row]')).toBeNull();
  });
});
