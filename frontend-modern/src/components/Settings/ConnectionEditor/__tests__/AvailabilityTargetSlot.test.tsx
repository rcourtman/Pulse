import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AvailabilityTargetsAPI } from '@/api/availabilityTargets';
import { AvailabilityTargetSlot } from '../CredentialSlots/AvailabilityTargetSlot';

vi.mock('@/api/availabilityTargets', () => ({
  AvailabilityTargetsAPI: {
    create: vi.fn(),
    list: vi.fn(),
    remove: vi.fn(),
    test: vi.fn(),
    testSaved: vi.fn(),
    update: vi.fn(),
  },
}));

const mockedCreate = vi.mocked(AvailabilityTargetsAPI.create);

describe('AvailabilityTargetSlot', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedCreate.mockResolvedValue({
      id: 'target-1',
      name: 'Rack sensor',
      address: 'rack-sensor.local',
      targetKind: 'device',
      protocol: 'tcp',
      port: 6053,
      enabled: true,
    });
  });

  afterEach(() => cleanup());

  it('prefills ESPHome devices as TCP availability targets', async () => {
    const onSaved = vi.fn();
    render(() => <AvailabilityTargetSlot onCancel={vi.fn()} onSaved={onSaved} />);

    fireEvent.change(screen.getByLabelText('Preset'), {
      target: { value: 'esphome-device' },
    });

    await waitFor(() => expect(screen.getByLabelText('Probe')).toHaveValue('tcp'));
    expect(screen.getByLabelText('Target type')).toHaveValue('device');
    expect(screen.getByLabelText('Port')).toHaveValue('6053');

    fireEvent.input(screen.getByLabelText('Name'), {
      target: { value: 'Rack sensor' },
    });
    fireEvent.input(screen.getByPlaceholderText('sensor.local'), {
      target: { value: 'rack-sensor.local' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add target' }));

    await waitFor(() =>
      expect(mockedCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Rack sensor',
          targetKind: 'device',
          address: 'rack-sensor.local',
          protocol: 'tcp',
          port: 6053,
          enabled: true,
        }),
      ),
    );
    expect(onSaved).toHaveBeenCalledTimes(1);
  });
});
