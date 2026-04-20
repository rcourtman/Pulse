import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { ConnectionsExplainer } from '../ConnectionsExplainer';

const DISMISS_KEY = 'pulse.infrastructure.explainer.dismissed.v1';

describe('ConnectionsExplainer', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    window.localStorage.clear();
  });

  it('names both ingestion modes with their branded labels', () => {
    render(() => <ConnectionsExplainer />);

    expect(screen.getByText('Platform API')).toBeInTheDocument();
    expect(screen.getByText('Pulse Unified Agent')).toBeInTheDocument();
  });

  it('frames the agent as supplementary to the API, not a replacement', () => {
    render(() => <ConnectionsExplainer />);

    expect(screen.queryByText(/Recommended/i)).toBeNull();
    expect(screen.getByText(/supplements the API/i)).toBeInTheDocument();
    expect(screen.getByText(/Primary source for workloads/i)).toBeInTheDocument();
  });

  it('calls out that Assistant / Patrol command execution is opt-in, not default', () => {
    render(() => <ConnectionsExplainer />);

    expect(screen.getByText(/off by default, opt in per host/i)).toBeInTheDocument();
    for (const capability of ['Assistant commands', 'Patrol remediation']) {
      expect(screen.getByText(capability)).toBeInTheDocument();
    }
    // The old "Always on: Hardware metrics" framing read as unavoidable
    // surveillance. The agent paragraph already names what it collects;
    // we don't restate it as an always-on chip.
    expect(screen.queryByText(/^Always on$/i)).toBeNull();
    expect(screen.queryByText('Hardware metrics')).toBeNull();
  });

  it('surfaces trust facts users care about', () => {
    render(() => <ConnectionsExplainer />);

    for (const fact of ['Single Go binary', '~13 MB download', 'No runtime dependencies', 'Open source']) {
      expect(screen.getByText(fact)).toBeInTheDocument();
    }
  });

  it('hides itself and persists dismissal to localStorage when closed', () => {
    const { container } = render(() => <ConnectionsExplainer />);

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }));

    expect(container.querySelector('section')).toBeNull();
    expect(window.localStorage.getItem(DISMISS_KEY)).toBe('1');
  });

  it('stays dismissed across remounts once localStorage records it', () => {
    window.localStorage.setItem(DISMISS_KEY, '1');
    const { container } = render(() => <ConnectionsExplainer />);

    expect(container.querySelector('section')).toBeNull();
  });
});
