import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { InfrastructureSourcePicker } from '../InfrastructureSourcePicker';

describe('InfrastructureSourcePicker', () => {
  afterEach(() => {
    cleanup();
  });

  it('labels admitted VMware as Early support and keeps Pulse Agent on the host profile path', () => {
    render(() => <InfrastructureSourcePicker onSelectStep={vi.fn()} />);

    expect(screen.getByPlaceholderText('Search platforms, hosts, services...')).toBeInTheDocument();
    // Picker now leads with the two primary paths (API detect / agent
    // install); the per-platform card grid below is framed as the
    // alternative.
    expect(screen.getByText('Choose how Pulse should connect')).toBeInTheDocument();
    expect(screen.getByText('Or pick a specific platform')).toBeInTheDocument();
    // VMware lives behind 'Show more platforms' since it is a less common
    // homelab choice; expand the long tail to verify the Early support
    // badge still renders for it.
    fireEvent.click(screen.getByRole('button', { name: /Show \d+ more platforms?/i }));
    expect(screen.getByText('Early support')).toBeInTheDocument();
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

    fireEvent.input(screen.getByPlaceholderText('Search platforms, hosts, services...'), {
      target: { value: 'nas' },
    });

    expect(screen.getByText('Matching choices')).toBeInTheDocument();
    expect(screen.getByText('TrueNAS SCALE')).toBeInTheDocument();
    expect(screen.getByText('Unraid')).toBeInTheDocument();
    expect(screen.queryByText('VMware vCenter')).toBeNull();
  });
});
