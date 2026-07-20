import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { PatrolIntelligenceState } from '../usePatrolIntelligenceState';
import { PatrolAutopilotAcknowledgementDialog } from '../PatrolAutopilotAcknowledgementDialog';

afterEach(cleanup);

describe('PatrolAutopilotAcknowledgementDialog', () => {
  it('requires explicit acceptance of the server acknowledgement version before activation', () => {
    const activate = vi.fn();
    const state = {
      autopilotDialogOpen: () => true,
      autopilotStatus: () => ({
        code: 'acknowledgement_required',
        active: false,
        currentVersion: 1,
        acceptedScope: ['policy_authorized_actions', 'outcome_truth_not_inferred'],
        acceptedLimits: {},
      }),
      isUpdatingAutonomy: () => false,
      setAutopilotDialogOpen: vi.fn(),
      acknowledgeAndActivateAutopilot: activate,
    } as unknown as PatrolIntelligenceState;
    render(() => <PatrolAutopilotAcknowledgementDialog state={state} />);
    const activateButton = screen.getByRole('button', {
      name: 'Record acknowledgement and activate',
    });
    expect(activateButton).toBeDisabled();
    expect(screen.getByText(/version 1 acknowledgement/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('checkbox'));
    expect(activateButton).toBeEnabled();
    fireEvent.click(activateButton);
    expect(activate).toHaveBeenCalledTimes(1);
  });
});
