import { cleanup, fireEvent, render, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { ProxmoxBackupServersTable } from '../ProxmoxBackupServersTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: (props: { resource: Resource }) => (
    <div data-testid="pbs-detail" data-resource-id={props.resource.id}>
      PBS details
    </div>
  ),
}));
afterEach(cleanup);

const server = (
  id = 'pbs-east',
  customUrl: string | undefined = 'https://pbs-east.example:8007',
): Resource => ({
  id,
  type: 'pbs',
  name: 'pbs-main',
  displayName: 'pbs-main',
  platformId: id,
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_790_000_000_000,
  customUrl,
  pbs: {
    instanceId: id,
    nodeName: 'host.example',
    linkedAgentId: 'agent-host',
    connectionHealth: 'healthy',
    datastores: [
      { name: 'archive', total: 1000, used: 400, available: 600 },
      { name: 'main', total: 1000, used: 200, available: 800 },
    ],
  },
});
const host: Resource = {
  id: 'host',
  type: 'agent',
  name: 'pbs-main',
  displayName: 'pbs-main',
  platformId: 'host',
  platformType: 'proxmox-pbs',
  sourceType: 'agent',
  status: 'online',
  lastSeen: 1_790_000_000_000,
  agent: { agentId: 'agent-host', hostname: 'host.example' },
  customUrl: 'https://UNRELATED-HOST.example:9000',
};

describe('PBS Backups web-interface parity', () => {
  it.each([320, 390, 768, 1365])(
    'uses the service-owned URL on every datastore at width %s without opening details',
    (width) => {
      const ancestorKeydown = vi.fn();
      const { container } = render(() => (
        <div onKeyDown={ancestorKeydown}>
          <ProxmoxBackupServersTable servers={[server(), host]} layoutWidth={() => width} />
        </div>
      ));
      const links = within(container).getAllByRole('link', {
        name: 'Open web interface for pbs-main',
      });
      expect(links).toHaveLength(2);
      for (const link of links) {
        expect(link).toHaveAttribute('href', 'https://pbs-east.example:8007');
        expect(link).toHaveAttribute('target', '_blank');
        expect(link).toHaveAttribute('rel', 'noopener noreferrer');
        // Solid delegates key events: check its actual ancestor handler, not a
        // native intermediate listener that runs before delegated dispatch.
        link.addEventListener('click', (event) => event.preventDefault());
        fireEvent.keyDown(link, { key: 'Enter' });
        fireEvent.click(link);
        expect(ancestorKeydown).not.toHaveBeenCalled();
        expect(within(container).queryByTestId('pbs-detail')).not.toBeInTheDocument();
      }
      expect(container.innerHTML).not.toContain('UNRELATED-HOST');
      fireEvent.click(
        within(container).getAllByRole('button', { name: 'Expand details for pbs-main' })[0],
      );
      expect(within(container).getByTestId('pbs-detail')).toHaveAttribute(
        'data-resource-id',
        'pbs-east',
      );
    },
  );

  it('keeps same-labelled services and reordered datastores bound to their own endpoints', async () => {
    const east = server();
    const west = server('pbs-west', 'http://[2001:db8::2]:8007/proxy/');
    const [servers, setServers] = createSignal([east, west, host]);
    const { container } = render(() => <ProxmoxBackupServersTable servers={servers()} />);
    const hrefs = () =>
      within(container)
        .getAllByRole('link')
        .map((a) => a.getAttribute('href'));
    expect(hrefs()).toEqual([east.customUrl, east.customUrl, west.customUrl, west.customUrl]);
    setServers([
      { ...west, pbs: { ...west.pbs!, datastores: [...west.pbs!.datastores!].reverse() } },
      east,
    ]);
    await waitFor(() =>
      expect(hrefs()).toEqual([east.customUrl, east.customUrl, west.customUrl, west.customUrl]),
    );
  });

  it.each([
    'javascript:alert(1)',
    'data:text/html,test',
    '//pbs.example:8007',
    '/pbs',
    'not a URL',
  ])('warns rather than navigating an unsafe persisted URL %s', (url) => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={[server('pbs-east', url)]} />
    ));
    expect(within(container).queryByRole('link')).not.toBeInTheDocument();
    expect(
      within(container).getAllByRole('img', { name: 'Web interface URL for pbs-main is invalid' }),
    ).toHaveLength(2);
    expect(within(container).getAllByText('pbs-main')).toHaveLength(2);
  });

  it('withdraws an edited or cleared URL without replacing it with a host, name or remembered endpoint', async () => {
    const pbs = server();
    const [servers, setServers] = createSignal([pbs, host]);
    const { container } = render(() => <ProxmoxBackupServersTable servers={servers()} />);
    expect(within(container).getAllByRole('link')).toHaveLength(2);
    setServers([{ ...pbs, customUrl: 'javascript:alert(1)' }, host]);
    await waitFor(() => expect(within(container).queryByRole('link')).not.toBeInTheDocument());
    setServers([{ ...pbs, customUrl: '' }, host]);
    await waitFor(() => expect(within(container).queryByRole('img')).not.toBeInTheDocument());
    expect(within(container).queryByRole('link')).not.toBeInTheDocument();
    setServers([{ ...pbs, customUrl: ' https://new.example:8443/pbs/ ' }]);
    await waitFor(() =>
      expect(within(container).getAllByRole('link')[0]).toHaveAttribute(
        'href',
        'https://new.example:8443/pbs/',
      ),
    );
  });

  it('does not invent an endpoint when a server has no custom URL or datastore', () => {
    const pbs = { ...server(), customUrl: undefined, pbs: { ...server().pbs!, datastores: [] } };
    const { container } = render(() => <ProxmoxBackupServersTable servers={[pbs, host]} />);
    expect(within(container).queryByRole('link')).not.toBeInTheDocument();
    expect(within(container).getByText('pbs-main')).toBeInTheDocument();
  });
});
