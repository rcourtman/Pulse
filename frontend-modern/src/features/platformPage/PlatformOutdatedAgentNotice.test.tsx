import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { PlatformOutdatedAgentNotice } from './PlatformOutdatedAgentNotice';

afterEach(() => {
  cleanup();
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
});
