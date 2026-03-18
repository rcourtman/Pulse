import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { SetupWizard } from '../SetupWizard';

vi.mock('../steps/WelcomeStep', () => ({
  WelcomeStep: (props: { onNext: () => void }) => (
    <button onClick={props.onNext}>Welcome next</button>
  ),
}));

vi.mock('../steps/SecurityStep', () => ({
  SecurityStep: (props: { onComplete: () => void; onBack: () => void }) => (
    <div>
      <button onClick={props.onBack}>Security back</button>
      <button onClick={props.onComplete}>Security complete</button>
    </div>
  ),
}));

vi.mock('../StepIndicator', () => ({
  StepIndicator: () => <div>Step indicator</div>,
}));

describe('SetupWizard', () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  afterEach(() => {
    cleanup();
  });

  it('sends the normal runtime path directly into infrastructure install after security setup', () => {
    const onComplete = vi.fn();

    render(() => <SetupWizard onComplete={onComplete} />);

    fireEvent.click(screen.getByRole('button', { name: 'Welcome next' }));
    expect(screen.getByText('Step indicator')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Security complete' }));

    expect(onComplete).toHaveBeenCalledWith('/settings/infrastructure/install');
  });
});
