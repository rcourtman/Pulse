import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { InfrastructureSourcePicker } from '../InfrastructureSourcePicker';

describe('InfrastructureSourcePicker', () => {
  afterEach(() => {
    cleanup();
  });

  it('labels admitted VMware as first-lab-ready and keeps Pulse Agent on the host profile path', () => {
    render(() => <InfrastructureSourcePicker onSelectType={vi.fn()} />);

    expect(screen.getByText('First lab ready')).toBeInTheDocument();
    expect(screen.queryByText('Available now')).toBeNull();
    expect(
      screen.getByText(
        'Linux, macOS, Windows, FreeBSD, and Unraid host/appliance profiles where you want low-overhead node-local telemetry.',
      ),
    ).toBeInTheDocument();
  });
});
