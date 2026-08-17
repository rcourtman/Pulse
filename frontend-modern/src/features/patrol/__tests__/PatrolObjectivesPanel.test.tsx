import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { PatrolObjectivesPanel } from '../PatrolObjectivesPanel';

const api = vi.hoisted(() => ({
  get: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
}));

vi.mock('@/api/patrol', () => ({
  getPatrolObjectives: api.get,
  createPatrolObjective: api.create,
  updatePatrolObjective: api.update,
  deletePatrolObjective: api.remove,
}));

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({ resources: () => [] }),
}));

vi.mock('@/components/Settings/ResourcePicker', () => ({
  ResourcePicker: () => <div data-testid="resource-picker">Resource picker</div>,
}));

vi.mock('@/utils/toast', () => ({ showSuccess: vi.fn(), showError: vi.fn() }));

const objective = {
  id: 'objective-1',
  brief: 'Keep cameras available',
  optional_context: 'Use local evidence',
  scope: { resource_ids: ['camera-1'] },
  status: 'active' as const,
  coverage: {
    state: 'covered' as const,
    reason_code: 'observer_healthy',
    summary: 'Observer is installed and reporting healthy local evidence.',
  },
  revision: 3,
  created_at: '2026-08-14T00:00:00Z',
  updated_at: '2026-08-14T00:00:00Z',
};

describe('PatrolObjectivesPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.get.mockResolvedValue([]);
    api.create.mockResolvedValue({ ...objective, id: 'created-objective', revision: 1 });
    api.update.mockResolvedValue({ ...objective, revision: 4 });
    api.remove.mockResolvedValue(undefined);
  });

  it('creates an abstract estate-wide objective from one simple statement', async () => {
    render(() => <PatrolObjectivesPanel />);
    expect(await screen.findByText('Choose what matters most')).toBeInTheDocument();

    fireEvent.click(screen.getAllByRole('button', { name: 'Add objective' }).at(-1)!);
    const outcome = screen.getByLabelText('What should Patrol keep true?');
    fireEvent.input(outcome, { target: { value: 'Keep Jellyfin playback from buffering' } });
    fireEvent.input(screen.getByLabelText('Useful context (optional)'), {
      target: { value: 'Prefer local event evidence' },
    });
    fireEvent.click(screen.getAllByRole('button', { name: 'Add objective' }).at(-1)!);

    await waitFor(() =>
      expect(api.create).toHaveBeenCalledWith({
        brief: 'Keep Jellyfin playback from buffering',
        optional_context: 'Prefer local event evidence',
        resource_ids: [],
      }),
    );
  });

  it('shows truthful coverage and supports pause, edit, and delete controls', async () => {
    api.get.mockResolvedValue([objective]);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    render(() => <PatrolObjectivesPanel />);

    expect(await screen.findByText('Keep cameras available')).toBeInTheDocument();
    expect(screen.getByText('Watching in background')).toBeInTheDocument();
    expect(screen.getByText(objective.coverage.summary)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Pause' }));
    await waitFor(() =>
      expect(api.update).toHaveBeenCalledWith('objective-1', { revision: 3, status: 'paused' }),
    );
    await waitFor(() => expect(screen.getByRole('button', { name: 'Pause' })).toBeEnabled());

    const editButton = screen.getByRole('button', { name: 'Edit' });
    fireEvent.click(editButton);
    expect(screen.getByRole('dialog', { name: 'Edit Patrol objective' })).toBeInTheDocument();
    expect(screen.getByDisplayValue('Keep cameras available')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Close objective dialog' }));
    await waitFor(() => expect(editButton).toHaveFocus());

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    await waitFor(() => expect(api.remove).toHaveBeenCalledWith('objective-1', 3));
  });

  it('does not present a healthy proxy signal as full objective coverage', async () => {
    api.get.mockResolvedValue([
      {
        ...objective,
        brief: 'Keep Jellyfin playback smooth',
        coverage: {
          state: 'uncovered',
          reason_code: 'observer_proxy',
          summary:
            'A healthy local signal is installed, but it does not directly measure the full objective.',
        },
      },
    ]);
    render(() => <PatrolObjectivesPanel />);

    expect(await screen.findByText('Keep Jellyfin playback smooth')).toBeInTheDocument();
    expect(screen.getByText('Useful signal only')).toBeInTheDocument();
    expect(screen.queryByText('Watching in background')).not.toBeInTheDocument();
    expect(
      screen.getByText(
        'A healthy local signal is installed, but it does not directly measure the full objective.',
      ),
    ).toBeInTheDocument();
  });

  it('does not convert a failed objectives read into a broken Patrol route', async () => {
    api.get.mockRejectedValue(new Error('offline'));
    render(() => <PatrolObjectivesPanel />);
    expect(await screen.findByText('Patrol objectives could not be loaded.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
  });
});
