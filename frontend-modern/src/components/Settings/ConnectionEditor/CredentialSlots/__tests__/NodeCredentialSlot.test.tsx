import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { InfrastructurePlatformSettingsProps } from '@/components/Settings/proxmoxSettingsModel';
import { NodeCredentialSlot } from '../NodeCredentialSlot';

const importApprovalRequiredMessage =
  'Approve the import plan before generating setup commands or adding this source.';

const createSettings = (): InfrastructurePlatformSettingsProps =>
  ({
    securityStatus: () => null,
    resolveTemperatureMonitoringEnabled: () => true,
    temperatureMonitoringLocked: () => false,
    savingTemperatureSetting: () => false,
    handleNodeTemperatureMonitoringChange: vi.fn().mockResolvedValue(undefined),
    handleTemperatureMonitoringChange: vi.fn().mockResolvedValue(undefined),
    saveNode: vi.fn().mockResolvedValue(undefined),
  }) as unknown as InfrastructurePlatformSettingsProps;

describe('NodeCredentialSlot', () => {
  afterEach(() => cleanup());

  it('requires candidate import plan approval before guided setup handoff or manual save', () => {
    render(() => (
      <NodeCredentialSlot
        nodeType="pve"
        settings={createSettings()}
        prefillNode={{
          type: 'pve',
          name: 'tower',
          host: 'https://tower.local:8006',
        }}
        importCandidate={{
          kind: 'discovery',
          server: {
            type: 'pve',
            ip: '10.0.0.10',
            hostname: 'tower.local',
            port: 8006,
            version: 'Proxmox VE',
            release: '9.0',
          },
        }}
        onCancel={vi.fn()}
        onSaved={vi.fn()}
      />
    ));

    expect(screen.getByText('Candidate import plan')).toBeInTheDocument();
    expect(screen.getByText('Proxmox VE 9.0')).toBeInTheDocument();

    const blockedSetupButton = screen.getAllByTitle(importApprovalRequiredMessage)[0];
    expect(blockedSetupButton).toBeDisabled();

    fireEvent.click(screen.getByLabelText(/Approve this import plan/i));

    expect(screen.getByTitle('Copy command')).not.toBeDisabled();

    fireEvent.click(screen.getByRole('button', { name: /^Manual Token Setup$/i }));

    const addButton = screen.getByRole('button', { name: 'Add node' });
    expect(addButton).toBeDisabled();

    fireEvent.click(screen.getByLabelText(/Approve this import plan/i));

    expect(addButton).not.toBeDisabled();
  });
});
