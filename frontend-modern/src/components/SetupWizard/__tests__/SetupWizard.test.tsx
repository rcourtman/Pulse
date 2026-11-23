import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import { SetupWizard } from '../SetupWizard';

vi.mock('../steps/WelcomeStep', () => ({
  WelcomeStep: (props: { onNext: () => void; windowExpired?: boolean }) => (
    <div>
      {props.windowExpired ? <span>Window expired notice</span> : null}
      <button onClick={props.onNext}>Welcome next</button>
    </div>
  ),
}));

vi.mock('../steps/SecurityStep', () => ({
  SecurityStep: (props: {
    onComplete: () => void;
    onBack: () => void;
    onRequireUnlock?: () => void;
    showBack?: boolean;
  }) => (
    <div>
      {props.showBack !== false ? <button onClick={props.onBack}>Security back</button> : null}
      <button onClick={props.onComplete}>Security complete</button>
      {props.onRequireUnlock ? (
        <button onClick={props.onRequireUnlock}>Require unlock</button>
      ) : null}
    </div>
  ),
}));

vi.mock('../SetupCompletionPanel', () => ({
  SetupCompletionPanel: (props: { onComplete: (nextPath?: string) => void }) => (
    <button onClick={() => props.onComplete('/settings/infrastructure?add=linux-host')}>
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
    setActiveLocale(DEFAULT_LOCALE);
    cleanup();
  });

  it('routes first-run setup through the completion handoff before infrastructure install', () => {
    const onComplete = vi.fn();

    render(() => <SetupWizard onComplete={onComplete} />);

    expect(
      screen.getByText('Step indicator 0:Unlock server > Security > First source'),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Welcome next' }));
    expect(
      screen.getByText('Step indicator 1:Unlock server > Security > First source'),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Security complete' }));
    expect(
      screen.getByText('Step indicator 2:Unlock server > Security > First source'),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Completion finish' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Completion finish' }));

    expect(onComplete).toHaveBeenCalledWith('/settings/infrastructure?add=linux-host');
  });

  it('passes localized first-session step labels into the shared progress indicator', () => {
    setActiveLocale('de');

    render(() => <SetupWizard onComplete={vi.fn()} />);

    expect(
      screen.getByText('Step indicator 0:Server entsperren > Sicherheit > Erste Quelle'),
    ).toBeInTheDocument();
  });

  it('scrolls back to the top when advancing between setup steps', () => {
    const scrollToSpy = vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
    try {
      render(() => <SetupWizard onComplete={vi.fn()} />);
      scrollToSpy.mockClear();

      fireEvent.click(screen.getByRole('button', { name: 'Welcome next' }));
      expect(scrollToSpy).toHaveBeenCalledWith({ top: 0, behavior: 'auto' });

      scrollToSpy.mockClear();
      fireEvent.click(screen.getByRole('button', { name: 'Security complete' }));
      expect(scrollToSpy).toHaveBeenCalledWith({ top: 0, behavior: 'auto' });
    } finally {
      scrollToSpy.mockRestore();
    }
  });

  it('starts directly on the 2-step security flow when the first-boot setup window is open', () => {
    const onComplete = vi.fn();

    render(() => <SetupWizard onComplete={onComplete} setupWindowOpen />);

    expect(screen.getByText('Step indicator 0:Security > First source')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Security back' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Security complete' }));
    expect(screen.getByText('Step indicator 1:Security > First source')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Completion finish' }));
    expect(onComplete).toHaveBeenCalledWith('/settings/infrastructure?add=linux-host');
  });

  it('falls back to the unlock step with an expiry notice if the setup window expires mid-flow', () => {
    render(() => <SetupWizard onComplete={vi.fn()} setupWindowOpen />);

    expect(screen.getByText('Step indicator 0:Security > First source')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Require unlock' }));

    expect(
      screen.getByText('Step indicator 0:Unlock server > Security > First source'),
    ).toBeInTheDocument();
    expect(screen.getByText('Window expired notice')).toBeInTheDocument();
  });
});
