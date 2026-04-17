import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ADD_SYSTEM_CHOICES, AddSystemPicker } from '../AddSystemPicker';

describe('AddSystemPicker', () => {
  afterEach(() => cleanup());

  it('does not render any dialog content when closed', () => {
    render(() => (
      <AddSystemPicker isOpen={false} onClose={() => {}} onSelect={() => {}} />
    ) as any);
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('lists one button per add-system choice including Proxmox VE, PBS, PMG, TrueNAS, VMware, and agent host', () => {
    render(() => (
      <AddSystemPicker isOpen={true} onClose={() => {}} onSelect={() => {}} />
    ) as any);

    for (const choice of ADD_SYSTEM_CHOICES) {
      expect(screen.getByText(choice.title)).toBeInTheDocument();
    }
    expect(ADD_SYSTEM_CHOICES.map((c) => c.kind)).toEqual([
      'pve',
      'pbs',
      'pmg',
      'truenas',
      'vmware',
      'agent',
    ]);
  });

  it('invokes onSelect with the matching choice when a tile is clicked', () => {
    const onSelect = vi.fn();
    render(() => (
      <AddSystemPicker isOpen={true} onClose={() => {}} onSelect={onSelect} />
    ) as any);

    fireEvent.click(screen.getByText('TrueNAS SCALE'));
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelect.mock.calls[0][0].kind).toBe('truenas');
    expect(onSelect.mock.calls[0][0].method).toBe('api');
  });

  it('invokes onClose when the close button is clicked', () => {
    const onClose = vi.fn();
    render(() => (
      <AddSystemPicker isOpen={true} onClose={onClose} onSelect={() => {}} />
    ) as any);

    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('marks the agent-host choice as an agent install and everything else as an API connection', () => {
    const agent = ADD_SYSTEM_CHOICES.find((c) => c.kind === 'agent');
    const apis = ADD_SYSTEM_CHOICES.filter((c) => c.kind !== 'agent');
    expect(agent?.method).toBe('agent');
    expect(apis.every((c) => c.method === 'api')).toBe(true);
  });
});
