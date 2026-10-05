import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { eventBus } from '@/stores/events';
import { PatrolSuppressionRules } from '../PatrolSuppressionRules';
import {
  deleteManualSuppressionRule,
  getSuppressionRules,
  type PatrolSuppressionRule,
} from '@/api/patrol';

const context = vi.hoisted(() => ({ org: 'tenant-a' }));
vi.mock('@/utils/apiClient', () => ({ getOrgID: () => context.org }));
vi.mock('@/api/patrol', async (original) => ({
  ...(await original<typeof import('@/api/patrol')>()),
  getSuppressionRules: vi.fn(),
  deleteManualSuppressionRule: vi.fn(),
}));
const read = vi.mocked(getSuppressionRules);
const remove = vi.mocked(deleteManualSuppressionRule);
const manual: PatrolSuppressionRule = {
  id: 'rule_vm-101_backup_1',
  resource_id: 'vm-101',
  resource_name: 'Database VM',
  category: 'backup',
  description: 'Backups are held off-site',
  created_from: 'manual',
  created_at: '2026-10-05T12:00:00Z',
};
const broad: PatrolSuppressionRule = {
  ...manual,
  id: 'rule_any_any_2',
  resource_id: '',
  resource_name: '',
  category: '',
  description: 'Intentional maintenance',
};
const dismissed: PatrolSuppressionRule = {
  ...manual,
  id: 'finding_important',
  finding_id: 'important',
  created_from: 'dismissed',
};
const legacy: PatrolSuppressionRule = { ...manual, id: 'rule_legacy', created_from: 'suppress' };
const initial = [manual, broad, dismissed, legacy];

async function openRules() {
  const view = render(() => <PatrolSuppressionRules />);
  const details = document.getElementById('patrol-suppression-rules') as HTMLDetailsElement;
  details.open = true;
  fireEvent(details, new Event('toggle'));
  await screen.findByRole('button', { name: 'Remove rule for Database VM, backup' });
  return view;
}
async function chooseManual() {
  fireEvent.click(screen.getByRole('button', { name: 'Remove rule for Database VM, backup' }));
  return screen.findByRole('dialog', { name: 'Remove suppression rule?' });
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

describe('Patrol manual-rule reversal', () => {
  beforeEach(() => {
    context.org = 'tenant-a';
    window.history.replaceState(null, '', '/patrol/activity');
    read.mockReset().mockResolvedValue(initial);
    remove.mockReset().mockResolvedValue(undefined);
  });
  afterEach(cleanup);

  it('does not load until requested; shows exact IDs, reasons, wildcard scope and non-manual distinction', async () => {
    render(() => <PatrolSuppressionRules />);
    expect(read).not.toHaveBeenCalled();
    const details = document.getElementById('patrol-suppression-rules') as HTMLDetailsElement;
    details.open = true;
    fireEvent(details, new Event('toggle'));
    await screen.findByText('Rule ID: rule_vm-101_backup_1');
    expect(screen.getByText('Resource ID: vm-101')).toBeInTheDocument();
    expect(screen.getByText('Backups are held off-site')).toBeInTheDocument();
    expect(
      screen.getByText('Broad rule: matches all resources in all categories.'),
    ).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /^Remove rule for/ })).toHaveLength(2);
    expect(screen.getByText(/2 other remembered decisions/)).toBeInTheDocument();
    expect(screen.queryByText('Rule ID: finding_important')).not.toBeInTheDocument();
    expect(read).toHaveBeenCalledWith('tenant-a', expect.any(AbortSignal));
  });

  it('confirms the exact scope with safe initial focus and permits cancellation without a mutation', async () => {
    await openRules();
    const trigger = screen.getByRole('button', { name: 'Remove rule for Database VM, backup' });
    trigger.focus();
    const dialog = await chooseManual();
    expect(within(dialog).getByText(manual.id)).toBeInTheDocument();
    expect(within(dialog).getByText(manual.description)).toBeInTheDocument();
    await waitFor(() =>
      expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus(),
    );
    fireEvent.keyDown(document, { key: 'Escape' });
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(remove).not.toHaveBeenCalled();
    expect(read).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(trigger).toHaveFocus());
  });

  it('keeps long reasons in a keyboard-scrollable region separate from the confirmation controls', async () => {
    const longReason = 'Keep the documented scope.\n'.repeat(30);
    read.mockResolvedValueOnce([{ ...manual, description: longReason }]);
    await openRules();
    const dialog = await chooseManual();
    const region = within(dialog).getByRole('region', { name: 'Rule scope and reason' });
    expect(region).toHaveAttribute('tabindex', '0');
    expect(within(region).getByText(manual.id)).toBeInTheDocument();
    expect(within(region).getByText(/Keep the documented scope/)).toBeInTheDocument();
    expect(region).not.toContainElement(within(dialog).getByRole('button', { name: 'Cancel' }));
    expect(region).not.toContainElement(
      within(dialog).getByRole('button', { name: 'Remove this rule' }),
    );
    await waitFor(() =>
      expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus(),
    );
    expect(remove).not.toHaveBeenCalled();
  });

  it('revalidates, deletes one exact manual ID and confirms absence while keeping unrelated decisions', async () => {
    await openRules();
    read.mockResolvedValueOnce(initial).mockResolvedValueOnce([broad, dismissed, legacy]);
    await chooseManual();
    fireEvent.click(screen.getByRole('button', { name: 'Remove this rule' }));
    await screen.findByRole('status');
    expect(remove).toHaveBeenCalledExactlyOnceWith(manual, 'tenant-a', expect.any(AbortSignal));
    expect(read).toHaveBeenCalledTimes(3);
    expect(screen.queryByText(`Rule ID: ${manual.id}`)).not.toBeInTheDocument();
    expect(screen.getByText(`Rule ID: ${broad.id}`)).toBeInTheDocument();
    expect(screen.getByText(/2 other remembered decisions/)).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent(
      'Dismissed findings and history were not reopened or removed',
    );
    await waitFor(() =>
      expect(
        document.getElementById('patrol-suppression-rules')?.querySelector('summary'),
      ).toHaveFocus(),
    );
  });

  it('shows all-resource/all-category scope again at confirmation', async () => {
    await openRules();
    fireEvent.click(
      screen.getByRole('button', { name: 'Remove rule for All resources, All categories' }),
    );
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('All resources')).toBeInTheDocument();
    expect(within(dialog).getByText('All categories')).toBeInTheDocument();
    expect(
      within(dialog).getByText('This broad rule covers all resources in all categories.'),
    ).toBeInTheDocument();
  });

  it.each([
    ['scope changed', [{ ...manual, category: 'security' }, broad]],
    ['manual origin changed', [{ ...manual, created_from: 'finding' }, broad]],
    ['already removed', [broad]],
  ])('stops without DELETE when the confirmed rule is %s', async (_, changed) => {
    await openRules();
    read.mockResolvedValueOnce(changed);
    await chooseManual();
    fireEvent.click(screen.getByRole('button', { name: 'Remove this rule' }));
    await screen.findByRole('alert');
    expect(remove).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: /^Remove rule for/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('invalidates the confirmation and old data on an organisation switch', async () => {
    await openRules();
    await chooseManual();
    context.org = 'tenant-b';
    eventBus.emit('org_switched', context.org);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.queryByText(`Rule ID: ${manual.id}`)).not.toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('Organisation or access changed');
    expect(remove).not.toHaveBeenCalled();
    read.mockResolvedValueOnce([]);
    fireEvent.click(screen.getByRole('button', { name: 'Reload rules' }));
    await screen.findByText('No manually created suppression rules.');
    expect(read).toHaveBeenLastCalledWith('tenant-b', expect.any(AbortSignal));
  });

  it('drops a late preflight after access revocation without deleting or reloading automatically', async () => {
    await openRules();
    const pending = deferred<PatrolSuppressionRule[]>();
    read.mockReturnValueOnce(pending.promise);
    await chooseManual();
    fireEvent.click(screen.getByRole('button', { name: 'Remove this rule' }));
    const signal = read.mock.lastCall![1]!;
    eventBus.emit('organizations_changed');
    pending.resolve(initial);
    await waitFor(() => expect(signal.aborted).toBe(true));
    expect(remove).not.toHaveBeenCalled();
    expect(read).toHaveBeenCalledTimes(2);
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('checks context even when a switch did not emit an event', async () => {
    await openRules();
    await chooseManual();
    context.org = 'tenant-b';
    fireEvent.click(screen.getByRole('button', { name: 'Remove this rule' }));
    expect(remove).not.toHaveBeenCalled();
    expect(read).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('alert')).toHaveTextContent('Reload rules');
  });

  it.each(['transport', '403'])(
    'requires explicit readback after %s uncertainty and never retries DELETE',
    async () => {
      await openRules();
      remove.mockRejectedValueOnce(new Error('Uncertain or refused request'));
      await chooseManual();
      fireEvent.click(screen.getByRole('button', { name: 'Remove this rule' }));
      await screen.findByRole('alert');
      expect(remove).toHaveBeenCalledTimes(1);
      expect(read).toHaveBeenCalledTimes(2);
      expect(screen.getByRole('alert')).toHaveTextContent('Removal could not be confirmed');
      expect(screen.queryByRole('status')).not.toBeInTheDocument();
      read.mockResolvedValueOnce([]);
      fireEvent.click(screen.getByRole('button', { name: 'Reload rules' }));
      await screen.findByText('No manually created suppression rules.');
      expect(remove).toHaveBeenCalledTimes(1);
      expect(screen.queryByRole('status')).not.toBeInTheDocument();
    },
  );

  it('does not claim success when readback fails or still contains the deleted ID', async () => {
    await openRules();
    await chooseManual();
    fireEvent.click(screen.getByRole('button', { name: 'Remove this rule' }));
    await screen.findByRole('alert');
    expect(remove).toHaveBeenCalledTimes(1);
    expect(read).toHaveBeenCalledTimes(3);
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('prevents duplicate submission and aborts late readback on disposal', async () => {
    const view = await openRules();
    const pending = deferred<PatrolSuppressionRule[]>();
    read.mockResolvedValueOnce(initial).mockReturnValueOnce(pending.promise);
    await chooseManual();
    fireEvent.click(screen.getByRole('button', { name: 'Remove this rule' }));
    await waitFor(() => expect(read).toHaveBeenCalledTimes(3));
    fireEvent.click(screen.getByRole('button', { name: 'Verifying removal…' }));
    expect(remove).toHaveBeenCalledTimes(1);
    const signal = read.mock.lastCall![1]!;
    view.unmount();
    expect(signal.aborted).toBe(true);
    pending.resolve([broad]);
  });

  it('opens a linked rule section on mount without making a mutation', async () => {
    window.history.replaceState(null, '', '/patrol/activity#patrol-suppression-rules');
    render(() => <PatrolSuppressionRules />);
    await screen.findByText(`Rule ID: ${manual.id}`);
    expect(document.getElementById('patrol-suppression-rules')).toHaveAttribute('open');
    expect(remove).not.toHaveBeenCalled();
  });

  it('honours router hash state before browser history has caught up', async () => {
    // Solid Router can mount the Activity view before its pushState effect.
    expect(window.location.hash).toBe('');
    render(() => <PatrolSuppressionRules openForLink />);
    await screen.findByText(`Rule ID: ${manual.id}`);
    expect(document.getElementById('patrol-suppression-rules')).toHaveAttribute('open');
    expect(read).toHaveBeenCalledTimes(1);
    expect(remove).not.toHaveBeenCalled();
  });
});
