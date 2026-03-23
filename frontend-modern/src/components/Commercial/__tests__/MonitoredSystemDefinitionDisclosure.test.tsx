import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen } from '@solidjs/testing-library';

import { MonitoredSystemDefinitionDisclosure } from '../MonitoredSystemDefinitionDisclosure';

describe('MonitoredSystemDefinitionDisclosure', () => {
  it('shows concise copy first and reveals the full definition on demand', async () => {
    render(() => (
      <MonitoredSystemDefinitionDisclosure summary="Billing is based on monitored systems." />
    ));

    expect(screen.getByText('Billing is based on monitored systems.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'What counts?' })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    expect(
      screen.queryByText(/a monitored system is a top-level machine or cluster/i),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'What counts?' }));

    expect(screen.getByRole('button', { name: 'Hide details' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    expect(
      screen.getByText(/a monitored system is a top-level machine or cluster/i),
    ).toBeInTheDocument();
  });
});
