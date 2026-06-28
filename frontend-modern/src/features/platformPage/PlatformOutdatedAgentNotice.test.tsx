import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import { PlatformOutdatedAgentNotice } from './PlatformOutdatedAgentNotice';

afterEach(() => {
  cleanup();
  syncSessionPresentationPolicy(null);
});

describe('PlatformOutdatedAgentNotice', () => {
  it('renders nothing when no hosts are outdated', () => {
    render(() => (
      <PlatformOutdatedAgentNotice hosts={[]} targetVersion="v6.0.0-rc.6" missingLabel="images" />
    ));
    expect(screen.queryByTestId('platform-outdated-agent-notice')).not.toBeInTheDocument();
  });

  it('explains a single outdated host and what it is missing', () => {
    render(() => (
      <PlatformOutdatedAgentNotice
        hosts={[{ name: 'tower', version: 'v6.0.0-rc.5' }]}
        targetVersion="v6.0.0-rc.6"
        missingLabel="images, networks, and storage"
        actionHref="/settings/infrastructure"
        actionLabel="Open agent upgrade commands"
      />
    ));
    const notice = screen.getByTestId('platform-outdated-agent-notice');
    expect(notice).toHaveTextContent(
      'tower is running an older Pulse agent (v6.0.0-rc.5). Update it to v6.0.0-rc.6 to see images, networks, and storage for this host.',
    );
    expect(screen.getByRole('link', { name: 'Open agent upgrade commands' })).toHaveAttribute(
      'href',
      '/settings/infrastructure',
    );
  });

  it('summarises multiple outdated hosts and lists them', () => {
    render(() => (
      <PlatformOutdatedAgentNotice
        hosts={[
          { name: 'tower', version: 'v6.0.0-rc.5' },
          { name: 'delly', version: 'v6.0.0-rc.5' },
        ]}
        targetVersion="v6.0.0-rc.6"
        missingLabel="images, networks, and storage"
      />
    ));
    const notice = screen.getByTestId('platform-outdated-agent-notice');
    expect(notice).toHaveTextContent('2 hosts are running an older Pulse agent.');
    expect(notice).toHaveTextContent('Affected: tower, delly.');
  });

  it('uses latest-detail copy for hybrid platform pages', () => {
    render(() => (
      <PlatformOutdatedAgentNotice
        hosts={[{ name: 'delly', version: 'v5.1.34' }]}
        targetVersion="v6.0.0-rc.6"
        missingLabel="agent-contributed platform detail"
        copyVariant="latest-detail"
      />
    ));

    expect(screen.getByTestId('platform-outdated-agent-notice')).toHaveTextContent(
      'delly is running an older Pulse agent (v5.1.34). Update it to v6.0.0-rc.6 for the latest agent-contributed platform detail on this host.',
    );
  });

  it('hides agent upgrade commands when the session presentation policy hides upgrade prompts', () => {
    syncSessionPresentationPolicy({
      presentationPolicy: {
        demoMode: true,
        readOnly: true,
        hideCommercial: true,
        hideUpgrade: true,
      },
    });

    render(() => (
      <PlatformOutdatedAgentNotice
        hosts={[{ name: 'West Production A', version: 'v5.1.34' }]}
        targetVersion="v6.0.0-rc.7"
        missingLabel="agent-contributed Proxmox node detail and command support"
        copyVariant="latest-detail"
        actionHref="/settings/infrastructure?agentUpdates=1"
        actionLabel="Open agent upgrade commands"
      />
    ));

    expect(screen.queryByTestId('platform-outdated-agent-notice')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Open agent upgrade commands' })).toBeNull();
  });

  it('can describe stale in-guest agents on VMs without host copy', () => {
    render(() => (
      <PlatformOutdatedAgentNotice
        hosts={[
          { name: 'app-01', version: 'v5.1.34' },
          { name: 'db-01', version: 'v5.1.34' },
        ]}
        targetVersion="v6.0.0-rc.6"
        missingLabel="in-guest telemetry and command support"
        copyVariant="latest-detail"
        subjectSingular="VM"
        subjectPlural="VMs"
      />
    ));

    const notice = screen.getByTestId('platform-outdated-agent-notice');
    expect(notice).toHaveTextContent('2 VMs are running an older Pulse agent.');
    expect(notice).toHaveTextContent(
      'Update them to v6.0.0-rc.6 for the latest in-guest telemetry and command support.',
    );
    expect(notice).toHaveTextContent('Affected: app-01, db-01.');
  });
});
