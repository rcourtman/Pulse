import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen } from '@solidjs/testing-library';

import { MonitoredSystemDefinitionDisclosure } from '../MonitoredSystemDefinitionDisclosure';

describe('MonitoredSystemDefinitionDisclosure', () => {
  it('shows concise copy first and reveals the full definition on demand', async () => {
    render(() => <MonitoredSystemDefinitionDisclosure showSummary />);

    expect(
      screen.getByText(
        'Pulse counts top-level monitored systems. Child resources underneath them are included.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'View counting rules' })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    expect(
      screen.queryByText(/a monitored system is a top-level monitored root/i),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'View counting rules' }));

    expect(screen.getByRole('button', { name: 'Hide counting rules' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    expect(
      screen.getByText(/a monitored system is a top-level monitored root/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/docker host, kubernetes cluster, proxmox node/i)).toBeInTheDocument();
  });

  it('does not expose brief summary copy unless requested', () => {
    render(() => <MonitoredSystemDefinitionDisclosure />);

    expect(
      screen.queryByText(
        'Pulse counts top-level monitored systems. Child resources underneath them are included.',
      ),
    ).not.toBeInTheDocument();
  });

  it('supports route-driven default-open disclosure state', () => {
    render(() => <MonitoredSystemDefinitionDisclosure defaultOpen />);

    expect(screen.getByRole('button', { name: 'Hide counting rules' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    expect(
      screen.getByText(/a monitored system is a top-level monitored root/i),
    ).toBeInTheDocument();
  });
});
