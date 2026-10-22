import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, describe, expect, it } from 'vitest';

import { RecordOwnerLink, buildRecordOwnerPath } from '../RecordOwnerLink';

afterEach(() => {
  cleanup();
  window.history.replaceState(null, '', '/');
});

describe('buildRecordOwnerPath', () => {
  it('links where the owning page opens or filters to the resource', () => {
    expect(buildRecordOwnerPath({ id: 'pve:pve-a:100', name: 'web', type: 'vm' })).toBe(
      '/proxmox/overview?resource=pve%3Apve-a%3A100',
    );
    expect(buildRecordOwnerPath({ id: 'storage-1', name: 'local-zfs', type: 'storage' })).toBe(
      '/proxmox/storage?resource=storage-1',
    );
    expect(buildRecordOwnerPath({ id: 'vm-9', name: 'api', type: 'vmware-vm' })).toBe(
      '/vmware/overview?resource=vm-9',
    );
    expect(buildRecordOwnerPath({ id: 'pod-1', name: 'payments', type: 'pod' })).toBe(
      '/kubernetes/workloads?q=payments',
    );
    expect(buildRecordOwnerPath({ id: 'ctr-1', name: 'redis', type: 'app-container' })).toBe(
      '/docker/overview?q=redis',
    );
  });

  it('omits the link where the owning page can only show a whole tab', () => {
    expect(buildRecordOwnerPath({ id: 'ds-1', name: 'tank/apps', type: 'truenas-dataset' })).toBe(
      null,
    );
    expect(buildRecordOwnerPath({ id: 'host-1', name: 'esxi-01', type: 'vmware-host' })).toBe(null);
    expect(buildRecordOwnerPath({ id: 'img-1', name: 'nginx', type: 'docker-image' })).toBe(null);
    expect(buildRecordOwnerPath({ id: 'pbs-1', name: 'pbs', type: 'pbs' })).toBe(null);
    expect(buildRecordOwnerPath({ id: '', name: 'web', type: 'vm' })).toBe(null);
    expect(buildRecordOwnerPath(null)).toBe(null);
  });
});

describe('RecordOwnerLink', () => {
  it('navigates in the app to the owning resource', async () => {
    render(() => (
      <Router>
        <Route
          path="/*"
          component={() => (
            <RecordOwnerLink subject={{ id: 'pod-1', name: 'payments', type: 'pod' }} />
          )}
        />
      </Router>
    ));

    const link = screen.getByRole('link', { name: 'Open payments' });
    expect(link).toHaveAttribute('data-record-owner-link');
    await fireEvent.click(link);
    await waitFor(() => expect(window.location.pathname).toBe('/kubernetes/workloads'));
    expect(window.location.search).toBe('?q=payments');
  });

  it('renders nothing when the owner cannot be focused', () => {
    render(() => (
      <Router>
        <Route
          path="/*"
          component={() => (
            <RecordOwnerLink subject={{ id: 'ds-1', name: 'tank', type: 'truenas-dataset' }} />
          )}
        />
      </Router>
    ));

    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });
});
