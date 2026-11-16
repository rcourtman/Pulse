import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { eventBus } from '@/stores/events';
import type { DockerPortCheckSuggestion, DockerPortChecksResult } from '@/api/availabilityTargets';
import { DockerPortChecksNotice } from '../DockerPortChecksNotice';
import {
  offeredPortSuggestions,
  portLinesByHost,
  portOfferMessage,
  skippedPortLines,
  summarizePortChecksResult,
} from '../dockerPortChecksModel';

const { dockerPortSuggestionsMock, addDockerPortChecksMock, readOnlyMock, orgMock } = vi.hoisted(() => ({
  dockerPortSuggestionsMock: vi.fn(),
  addDockerPortChecksMock: vi.fn(),
  readOnlyMock: vi.fn(() => false),
  orgMock: vi.fn(() => 'default'),
}));

vi.mock('@/api/availabilityTargets', () => ({
  AvailabilityTargetsAPI: {
    dockerPortSuggestions: dockerPortSuggestionsMock,
    addDockerPortChecks: addDockerPortChecksMock,
  },
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsReadOnly: readOnlyMock,
}));

vi.mock('@/utils/apiClient', () => ({ getOrgID: orgMock }));

const port = (
  containerName: string,
  publicPort: number,
  overrides: Partial<DockerPortCheckSuggestion> = {},
): DockerPortCheckSuggestion => ({
  hostId: 'host-1',
  hostName: 'docker-vm',
  containerId: `id-${containerName}`,
  containerName,
  privatePort: publicPort,
  publicPort,
  address: '192.168.1.20',
  ...overrides,
});

const suggestions = [
  port('web', 8081),
  port('cache', 6379),
  port('admin', 9000, { address: undefined, skipReason: 'loopback_only' }),
  port('kv', 6380, { address: undefined, skipReason: 'not_http' }),
];

const result = (overrides: Partial<DockerPortChecksResult> = {}): DockerPortChecksResult => ({
  created: [],
  unanswered: [],
  skipped: [],
  remaining: 0,
  ...overrides,
});

describe('docker port check wording', () => {
  it('counts only ports Pulse can check and that were not dismissed', () => {
    expect(offeredPortSuggestions(suggestions, new Set()).map((s) => s.containerName)).toEqual([
      'web',
      'cache',
    ]);
    expect(
      offeredPortSuggestions(suggestions, new Set(['host-1/web/8081'])).map((s) => s.containerName),
    ).toEqual(['cache']);
    expect(portOfferMessage(1)).toBe('1 published container port has no availability check.');
    expect(portOfferMessage(2)).toBe('2 published container ports have no availability check.');
  });

  it('lists ports per host and says why a loopback port is left out', () => {
    const other = port('proxy', 443, { hostId: 'host-2', hostName: 'nas', address: '10.0.0.9' });
    const bound = port('dns', 5380, { address: '192.168.1.21' });
    expect(portLinesByHost([suggestions[0], suggestions[1], bound, other])).toEqual([
      'docker-vm at 192.168.1.20: web 8081, cache 6379, dns 5380 (192.168.1.21)',
      'nas at 10.0.0.9: proxy 443',
    ]);
    expect(skippedPortLines(suggestions)).toEqual([
      'Not a web port, so not offered: kv 6380.',
      'Published on 127.0.0.1 only, so Pulse cannot reach it: admin 9000.',
    ]);
  });

  it('reports what was added and what did not answer', () => {
    const added = summarizePortChecksResult(
      result({
        created: [{ id: 't1', name: 'web:8081', address: 'x', protocol: 'http', enabled: true }],
        unanswered: [{ ...port('dead', 8082), error: 'connection refused' }],
      }),
    );
    expect(added.tone).toBe('success');
    expect(added.message).toBe('Added 1 availability check.');
    expect(added.unansweredLine).toBe(
      '1 port did not answer over HTTP, so no check was added: dead 8082.',
    );

    const none = summarizePortChecksResult(
      result({ unanswered: [{ ...port('dead', 8082), error: 'connection refused' }] }),
    );
    expect(none.tone).toBe('info');
    expect(none.message).toBe('No availability checks were added.');
  });
});

describe('DockerPortChecksNotice', () => {
  beforeEach(() => {
    window.localStorage.clear();
    readOnlyMock.mockReturnValue(false);
    orgMock.mockReset().mockReturnValue('default');
    dockerPortSuggestionsMock.mockReset().mockResolvedValue(suggestions);
    addDockerPortChecksMock.mockReset();
  });
  afterEach(cleanup);

  it('offers the unchecked ports and names them on request', async () => {
    render(() => <DockerPortChecksNotice containerSignature="a" />);

    const offer = await screen.findByTestId('docker-port-checks-offer');
    expect(offer).toHaveTextContent('2 published container ports have no availability check.');
    expect(screen.queryByTestId('docker-port-checks-list')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Show ports' }));
    const list = screen.getByTestId('docker-port-checks-list');
    expect(list).toHaveTextContent('docker-vm at 192.168.1.20: web 8081, cache 6379');
    expect(list).toHaveTextContent('Published on 127.0.0.1 only');
  });

  it('adds checks, reports the outcome and stops offering a port that did not answer', async () => {
    const onChecksAdded = vi.fn();
    addDockerPortChecksMock.mockImplementation(async () => {
      // The server now reports only the port that did not answer.
      dockerPortSuggestionsMock.mockResolvedValue([port('cache', 6379)]);
      return result({
        created: [{ id: 't1', name: 'web:8081', address: 'x', protocol: 'http', enabled: true }],
        unanswered: [{ ...port('cache', 6379), error: 'connection refused' }],
      });
    });
    render(() => <DockerPortChecksNotice containerSignature="a" onChecksAdded={onChecksAdded} />);

    fireEvent.click(await screen.findByRole('button', { name: 'Add checks' }));

    const outcome = await screen.findByTestId('docker-port-checks-result');
    // Exactly the two ports on offer were sent, not the ones Pulse skips.
    expect(addDockerPortChecksMock).toHaveBeenCalledTimes(1);
    expect(
      (addDockerPortChecksMock.mock.calls[0][0] as DockerPortCheckSuggestion[]).map(
        (sent) => sent.containerName,
      ),
    ).toEqual(['web', 'cache']);
    expect(outcome).toHaveTextContent('Added 1 availability check.');
    expect(outcome).toHaveTextContent('1 port did not answer over HTTP');
    expect(screen.getByRole('link', { name: /View checks/ })).toBeInTheDocument();
    expect(onChecksAdded).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }));
    await waitFor(() => expect(screen.queryByTestId('docker-port-checks-result')).toBeNull());
    // The unanswered port is not offered again in this browser.
    expect(screen.queryByTestId('docker-port-checks-offer')).toBeNull();
  });

  it('stays dismissed until a new port is published', async () => {
    const view = render(() => <DockerPortChecksNotice containerSignature="a" />);
    await screen.findByTestId('docker-port-checks-offer');
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss the availability check offer' }));
    await waitFor(() => expect(screen.queryByTestId('docker-port-checks-offer')).toBeNull());
    view.unmount();

    render(() => <DockerPortChecksNotice containerSignature="a" />);
    await waitFor(() => expect(dockerPortSuggestionsMock).toHaveBeenCalledTimes(2));
    expect(screen.queryByTestId('docker-port-checks-offer')).toBeNull();
    cleanup();

    dockerPortSuggestionsMock.mockResolvedValue([...suggestions, port('new', 3000)]);
    render(() => <DockerPortChecksNotice containerSignature="b" />);
    expect(await screen.findByTestId('docker-port-checks-offer')).toHaveTextContent(
      '1 published container port has no availability check.',
    );
  });

  it('keeps the displayed selection across background inventory refreshes', async () => {
    const [signature, setSignature] = createSignal('a');
    render(() => <DockerPortChecksNotice containerSignature={signature()} />);
    await screen.findByTestId('docker-port-checks-offer');
    fireEvent.click(screen.getByRole('button', { name: 'Show ports' }));
    const listed = screen.getByTestId('docker-port-checks-list').textContent;

    dockerPortSuggestionsMock.mockResolvedValue([port('replacement', 9081)]);
    setSignature('b');
    await waitFor(() => expect(dockerPortSuggestionsMock).toHaveBeenCalledTimes(2));
    await Promise.resolve();
    expect(screen.getByTestId('docker-port-checks-list').textContent).toBe(listed);

    addDockerPortChecksMock.mockRejectedValueOnce(new Error('Published container ports changed.'));
    fireEvent.click(screen.getByRole('button', { name: 'Add checks' }));
    await screen.findByText('Could not add checks: Published container ports changed.');
    expect(addDockerPortChecksMock).toHaveBeenCalledWith(
      suggestions.slice(0, 2),
      expect.objectContaining({ expectedOrgID: 'default', retry: false }),
    );
    expect(screen.getByTestId('docker-port-checks-list').textContent).toBe(listed);
    expect(addDockerPortChecksMock).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss the availability check offer' }));
    await waitFor(() =>
      expect(screen.getByTestId('docker-port-checks-list')).toHaveTextContent('replacement 9081'),
    );
    expect(screen.queryByText('Could not add checks: Published container ports changed.')).toBeNull();
  });

  it('clears a retained offer when a later read is refused', async () => {
    const [signature, setSignature] = createSignal('a');
    render(() => <DockerPortChecksNotice containerSignature={signature()} />);
    await screen.findByTestId('docker-port-checks-offer');
    dockerPortSuggestionsMock.mockRejectedValueOnce(new Error('forbidden'));
    setSignature('b');
    await waitFor(() => expect(dockerPortSuggestionsMock).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.queryByTestId('docker-port-checks-offer')).toBeNull());
  });

  it('does not dispatch an old offer in a silently changed organization', async () => {
    render(() => <DockerPortChecksNotice containerSignature="a" />);
    await screen.findByTestId('docker-port-checks-offer');
    orgMock.mockReturnValue('other');
    dockerPortSuggestionsMock.mockResolvedValue([port('other-web', 9081)]);
    fireEvent.click(screen.getByRole('button', { name: 'Add checks' }));
    await waitFor(() => expect(dockerPortSuggestionsMock).toHaveBeenCalledTimes(2));
    expect(addDockerPortChecksMock).not.toHaveBeenCalled();
    expect(dockerPortSuggestionsMock).toHaveBeenLastCalledWith(
      expect.objectContaining({ expectedOrgID: 'other', retry: false }),
    );
  });

  it('retires an in-flight acknowledgement when the organization changes', async () => {
    let finish!: (value: DockerPortChecksResult) => void;
    addDockerPortChecksMock.mockImplementationOnce(
      () => new Promise<DockerPortChecksResult>((resolve) => (finish = resolve)),
    );
    const onChecksAdded = vi.fn();
    render(() => <DockerPortChecksNotice containerSignature="a" onChecksAdded={onChecksAdded} />);
    await screen.findByTestId('docker-port-checks-offer');
    fireEvent.click(screen.getByRole('button', { name: 'Add checks' }));
    const options = addDockerPortChecksMock.mock.calls[0][1];
    expect(options.expectedOrgID).toBe('default');

    orgMock.mockReturnValue('other');
    dockerPortSuggestionsMock.mockResolvedValue([port('other-web', 9081)]);
    eventBus.emit('org_switched', 'other');
    await waitFor(() => expect(dockerPortSuggestionsMock).toHaveBeenCalledTimes(2));
    expect(options.signal.aborted).toBe(true);
    finish(result({ created: [{ id: 'old', name: 'old', address: 'x', protocol: 'http', enabled: true }] }));
    await Promise.resolve();
    await Promise.resolve();
    expect(onChecksAdded).not.toHaveBeenCalled();
    expect(screen.queryByTestId('docker-port-checks-result')).toBeNull();
    expect(window.localStorage.getItem('pulse.dockerPortChecks.dismissed')).toBeNull();
    expect(screen.getByRole('button', { name: 'Add checks' })).toBeEnabled();
  });

  it('does not restore a retired membership read over its replacement', async () => {
    let finish!: (value: DockerPortCheckSuggestion[]) => void;
    dockerPortSuggestionsMock.mockImplementationOnce(
      () => new Promise<DockerPortCheckSuggestion[]>((resolve) => (finish = resolve)),
    );
    render(() => <DockerPortChecksNotice containerSignature="a" />);
    await waitFor(() => expect(dockerPortSuggestionsMock).toHaveBeenCalledTimes(1));
    const oldOptions = dockerPortSuggestionsMock.mock.calls[0][0];
    dockerPortSuggestionsMock.mockResolvedValue([port('current', 9081)]);
    eventBus.emit('organizations_changed');
    await screen.findByTestId('docker-port-checks-offer');
    fireEvent.click(screen.getByRole('button', { name: 'Show ports' }));
    expect(screen.getByTestId('docker-port-checks-list')).toHaveTextContent('current 9081');
    expect(oldOptions.signal.aborted).toBe(true);
    finish(suggestions);
    await Promise.resolve();
    await Promise.resolve();
    expect(screen.getByTestId('docker-port-checks-list')).toHaveTextContent('current 9081');
    expect(screen.getByTestId('docker-port-checks-list')).not.toHaveTextContent('cache 6379');
  });

  it('shows nothing to a read-only viewer or when the server refuses', async () => {
    readOnlyMock.mockReturnValue(true);
    render(() => <DockerPortChecksNotice containerSignature="a" />);
    await Promise.resolve();
    expect(dockerPortSuggestionsMock).not.toHaveBeenCalled();
    expect(screen.queryByTestId('docker-port-checks-offer')).toBeNull();
    cleanup();

    readOnlyMock.mockReturnValue(false);
    dockerPortSuggestionsMock.mockRejectedValue(new Error('forbidden'));
    render(() => <DockerPortChecksNotice containerSignature="a" />);
    await waitFor(() => expect(dockerPortSuggestionsMock).toHaveBeenCalled());
    expect(screen.queryByTestId('docker-port-checks-offer')).toBeNull();
  });
});
