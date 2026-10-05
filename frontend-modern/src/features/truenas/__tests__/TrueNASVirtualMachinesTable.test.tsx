import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { TrueNASVirtualMachinesTable } from '@/features/truenas/TrueNASVirtualMachinesTable';
import type { Resource } from '@/types/resource';

const makeVM = (
  id: string,
  vm: NonNullable<NonNullable<Resource['truenas']>['vm']>,
  overrides: Partial<Resource> = {},
): Resource =>
  ({
    id,
    type: 'vm',
    name: id,
    displayName: id,
    status: vm.state === 'RUNNING' ? 'online' : 'offline',
    platformType: 'truenas',
    platformScopes: ['truenas'],
    truenas: { vm: { name: id, ...vm } },
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('TrueNASVirtualMachinesTable', () => {
  it('flags only the stopped VMs nobody chose to stop and keeps configuration in the drawer', () => {
    const vms = [
      makeVM('windows-lab', {
        state: 'RUNNING',
        autostart: true,
        bootloader: 'UEFI',
        diskCount: 1,
        nicCount: 1,
      }),
      makeVM('ubuntu-build', { state: 'STOPPED', bootloader: 'UEFI', diskCount: 1 }),
      makeVM('router', { state: 'STOPPED', autostart: true, bootloader: 'UEFI' }),
      makeVM('lab-paused', { state: 'PAUSED' }),
    ];
    const { container } = render(() => (
      <TrueNASVirtualMachinesTable
        vms={vms}
        scope={vms}
        emptyIcon={<span />}
        emptyTitle="No VMs"
        emptyDescription="No VMs"
        showToolbar={false}
      />
    ));

    const noteFor = (name: string) =>
      screen.getByText(name).closest('tr')?.querySelector('[data-truenas-vm-state-note]');

    expect(noteFor('router')).toHaveTextContent('Stoppedshould be running');
    expect(noteFor('router')).toHaveAttribute('title', 'Set to start at boot but stopped');
    expect(noteFor('ubuntu-build')).toBeNull();
    expect(noteFor('windows-lab')).toBeNull();
    expect(noteFor('lab-paused')).toHaveAttribute('title', 'TrueNAS reports Paused');

    const headers = [...container.querySelectorAll('thead th')].map((th) => th.textContent?.trim());
    expect(headers).not.toContain('Boot');
    expect(headers).not.toContain('Devices');
    expect(container.querySelector('tbody')).not.toHaveTextContent('UEFI');
  });
});
