import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { PlatformOutdatedSensorSetupNotice } from './PlatformOutdatedSensorSetupNotice';

afterEach(() => {
  cleanup();
});

describe('PlatformOutdatedSensorSetupNotice', () => {
  it('renders nothing when no nodes are affected', () => {
    render(() => <PlatformOutdatedSensorSetupNotice nodes={[]} />);
    expect(screen.queryByTestId('platform-outdated-sensor-setup-notice')).not.toBeInTheDocument();
  });

  it('explains a single affected node and how to fix it', () => {
    render(() => (
      <PlatformOutdatedSensorSetupNotice
        nodes={[{ id: 'node-1', name: 'pve1' }]}
        actionHref="/settings/infrastructure"
      />
    ));
    const notice = screen.getByTestId('platform-outdated-sensor-setup-notice');
    expect(notice).toHaveTextContent(
      'pve1 is using an older temperature monitoring setup that cannot read SATA/SAS disk temperatures. Re-run the node setup script to upgrade it.',
    );
    expect(screen.getByRole('link', { name: 'Open Infrastructure settings' })).toHaveAttribute(
      'href',
      '/settings/infrastructure',
    );
  });

  it('names a renamed node by its table label and its Proxmox node', () => {
    render(() => (
      <PlatformOutdatedSensorSetupNotice
        nodes={[{ id: 'node-3', name: 'West Production C', nodeName: 'pve3' }]}
      />
    ));
    expect(screen.getByTestId('platform-outdated-sensor-setup-notice')).toHaveTextContent(
      'West Production C (pve3) is using an older temperature monitoring setup that cannot read SATA/SAS disk temperatures.',
    );
  });

  it('summarises multiple affected nodes and lists them', () => {
    render(() => (
      <PlatformOutdatedSensorSetupNotice
        nodes={[
          { id: 'node-1', name: 'pve1' },
          { id: 'node-3', name: 'West Production C', nodeName: 'pve3' },
        ]}
      />
    ));
    const notice = screen.getByTestId('platform-outdated-sensor-setup-notice');
    expect(notice).toHaveTextContent(
      '2 nodes are using an older temperature monitoring setup that cannot read SATA/SAS disk temperatures.',
    );
    expect(notice).toHaveTextContent('Affected: pve1, West Production C (pve3).');
  });
});
