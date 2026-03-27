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

vi.mock('../SetupCompletionPanel', () => ({
  SetupCompletionPanel: (props: { onComplete: (nextPath?: string) => void }) => (
    <button onClick={() => props.onComplete('/settings/infrastructure/install')}>
      Completion finish
    </button>
  ),
}));

vi.mock('../StepIndicator', () => ({
  StepIndicator: (props: { steps: string[]; currentStep: number }) => (
    <div>
      Step indicator {props.currentStep}:{props.steps.join(' > ')}
    </div>
  ),
}));

describe('SetupWizard', () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  afterEach(() => {
    cleanup();
  });

  it('routes first-run setup through the completion handoff before infrastructure install', () => {
    const onComplete = vi.fn();

    render(() => <SetupWizard onComplete={onComplete} />);

    expect(screen.getByText('Step indicator 0:Unlock > Security > Install')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Welcome next' }));
    expect(screen.getByText('Step indicator 1:Unlock > Security > Install')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Security complete' }));
    expect(screen.getByText('Step indicator 2:Unlock > Security > Install')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Completion finish' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Completion finish' }));

    expect(onComplete).toHaveBeenCalledWith('/settings/infrastructure/install');
  });
});
