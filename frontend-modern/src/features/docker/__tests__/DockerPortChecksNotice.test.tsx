import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { DockerPortCheckSuggestion, DockerPortChecksResult } from '@/api/availabilityTargets';
import { DockerPortChecksNotice } from '../DockerPortChecksNotice';
import {
  offeredPortSuggestions,
  portLinesByHost,
  portOfferMessage,
  skippedPortLines,
  summarizePortChecksResult,
} from '../dockerPortChecksModel';

const { dockerPortSuggestionsMock, addDockerPortChecksMock, readOnlyMock } = vi.hoisted(() => ({
  dockerPortSuggestionsMock: vi.fn(),
  addDockerPortChecksMock: vi.fn(),
  readOnlyMock: vi.fn(() => false),
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
