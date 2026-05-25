import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { InfrastructureSourcePicker } from '../InfrastructureSourcePicker';

describe('InfrastructureSourcePicker', () => {
  afterEach(() => {
    cleanup();
  });

  it('labels admitted VMware as Preview and keeps Pulse Agent on the host profile path', () => {
    render(() => <InfrastructureSourcePicker onSelectStep={vi.fn()} />);

    expect(screen.getByPlaceholderText('Search sources, devices, services...')).toBeInTheDocument();
    // Picker now leads with API detection and agent installation; endpoint
    // probes live under Settings -> Monitoring -> Availability checks.
    expect(screen.getByText('Choose how Pulse should connect')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Monitor network endpoint/i })).toBeNull();
    expect(screen.getByText('Or pick a specific source')).toBeInTheDocument();
    // VMware lives behind 'Show more sources' since it is a less common
    // homelab choice; expand the long tail to verify the Preview badge still
    // renders for it.
    fireEvent.click(screen.getByRole('button', { name: /Show \d+ more sources?/i }));
    expect(screen.getByText('Preview')).toBeInTheDocument();
    expect(screen.queryByText('Available now')).toBeNull();
    expect(screen.getByText('Unraid')).toBeInTheDocument();
    expect(screen.getByText('Linux, macOS, Windows host')).toBeInTheDocument();
    expect(screen.getByText('Array health, disks, Docker, host telemetry')).toBeInTheDocument();
    expect(screen.queryByText('Agent telemetry')).toBeNull();
  });

  it('routes user-facing catalog choices into the canonical setup flow', () => {
    const onSelectStep = vi.fn();
    render(() => <InfrastructureSourcePicker onSelectStep={onSelectStep} />);

    const unraidButton = screen.getByText('Unraid').closest('button');
    expect(unraidButton).not.toBeNull();
    fireEvent.click(unraidButton!);
    expect(onSelectStep).toHaveBeenLastCalledWith('unraid');

    const trueNasButton = screen.getByText('TrueNAS SCALE').closest('button');
    expect(trueNasButton).not.toBeNull();
    fireEvent.click(trueNasButton!);
    expect(onSelectStep).toHaveBeenLastCalledWith('truenas');
  });

  it('filters the catalog by user-recognizable names and aliases', () => {
    render(() => <InfrastructureSourcePicker onSelectStep={vi.fn()} />);

    fireEvent.input(screen.getByPlaceholderText('Search sources, devices, services...'), {
      target: { value: 'nas' },
    });

    expect(screen.getByText('Matching choices')).toBeInTheDocument();
    expect(screen.getByText('TrueNAS SCALE')).toBeInTheDocument();
    expect(screen.getByText('Unraid')).toBeInTheDocument();
    expect(screen.queryByText('VMware vCenter')).toBeNull();

    fireEvent.input(screen.getByPlaceholderText('Search sources, devices, services...'), {
      target: { value: 'mqtt' },
    });

    expect(screen.queryByText('Network endpoint')).toBeNull();
  });
});
