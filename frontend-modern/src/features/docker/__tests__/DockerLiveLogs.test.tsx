import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, expect, it, vi } from 'vitest';
import { eventBus } from '@/stores/events';
import type { Resource } from '@/types/resource';
import { DockerLiveLogs } from '../DockerLiveLogs';

const { read } = vi.hoisted(() => ({ read: vi.fn() }));
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: read, getOrgID: () => 'org' }));
vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsReadOnly: () => false,
}));
const container = { id: 'web', name: 'web', docker: { containerId: 'exact' } } as Resource;

afterEach(() => {
  cleanup();
  read.mockReset();
});
it('reads only on request and discards a response after the tenant changes', async () => {
  let deliver!: (result: { lines: string[] }) => void;
  read.mockImplementation(
    () =>
      new Promise((resolve) => {
        deliver = resolve;
      }),
  );
  render(() => <DockerLiveLogs hostId="east" enabled={true} containers={[container]} />);
  const panel = screen.getByText('Live container logs').closest('details')!;
  panel.open = true;
  expect(read).not.toHaveBeenCalled();
  await fireEvent.click(screen.getByRole('button', { name: 'Follow logs' }));
  const options = read.mock.calls[0][1];
  expect(JSON.parse(options.body)).toEqual({ hostId: 'east', containerId: 'exact' });
  expect(options.expectedOrgID).toBe('org');
  eventBus.emit('org_switched', 'other');
  expect(options.signal.aborted).toBe(true);
  deliver({ lines: ['old tenant log'] });
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Follow logs' })).toBeInTheDocument(),
  );
  expect(screen.queryByText('old tenant log')).not.toBeInTheDocument();
});
