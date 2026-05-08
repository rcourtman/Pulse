import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { InfrastructureSourcePicker } from '../InfrastructureSourcePicker';

describe('InfrastructureSourcePicker', () => {
  afterEach(() => {
    cleanup();
  });

  it('labels admitted VMware as first-lab-ready and keeps Pulse Agent on the host profile path', () => {
    render(() => <InfrastructureSourcePicker onSelectStep={vi.fn()} />);

    expect(screen.getByPlaceholderText('Search platforms, hosts, services...')).toBeInTheDocument();
    expect(screen.getByText('Common choices')).toBeInTheDocument();
    expect(screen.getByText('First lab ready')).toBeInTheDocument();
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
