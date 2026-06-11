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

  it('summarises multiple affected nodes and lists them', () => {
    render(() => (
      <PlatformOutdatedSensorSetupNotice
        nodes={[
          { id: 'node-1', name: 'pve1' },
          { id: 'node-2', name: 'pve2' },
        ]}
      />
    ));
    const notice = screen.getByTestId('platform-outdated-sensor-setup-notice');
    expect(notice).toHaveTextContent(
      '2 nodes are using an older temperature monitoring setup that cannot read SATA/SAS disk temperatures.',
    );
    expect(notice).toHaveTextContent('Affected: pve1, pve2.');
  });
});
