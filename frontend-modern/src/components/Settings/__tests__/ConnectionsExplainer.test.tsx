import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ConnectionsExplainer } from '../ConnectionsExplainer';

describe('ConnectionsExplainer', () => {
  afterEach(() => {
    cleanup();
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

  it('exposes Add connection and Install agent as the entry CTAs, not as duplicate buttons elsewhere', () => {
    const onAddConnection = vi.fn();
    const onInstallAgent = vi.fn();

    render(() => (
      <ConnectionsExplainer
        onAddConnection={onAddConnection}
        onInstallAgent={onInstallAgent}
      />
    ));

    const addBtn = screen.getByRole('button', { name: /^Add connection$/i });
    const installBtn = screen.getByRole('button', { name: /^Install agent$/i });

    fireEvent.click(addBtn);
    expect(onAddConnection).toHaveBeenCalledTimes(1);
    expect(onInstallAgent).not.toHaveBeenCalled();

    fireEvent.click(installBtn);
    expect(onInstallAgent).toHaveBeenCalledTimes(1);
  });

  it('does not render the action CTAs in read-only mode', () => {
    render(() => (
      <ConnectionsExplainer
        readOnly
        onAddConnection={() => {}}
        onInstallAgent={() => {}}
      />
    ));

    expect(screen.queryByRole('button', { name: /^Add connection$/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /^Install agent$/i })).toBeNull();
  });

  it('no longer offers a dismiss affordance — the cards are the entry path, not a tutorial banner', () => {
    render(() => <ConnectionsExplainer />);

    expect(screen.queryByRole('button', { name: /Dismiss/i })).toBeNull();
  });
});
