import { Component, createEffect, createMemo, createSignal, Show } from 'solid-js';
import { t } from '@/i18n';
import { WelcomeStep } from './steps/WelcomeStep';
import { SecurityStep } from './steps/SecurityStep';
import { SetupCompletionPanel } from './SetupCompletionPanel';
import { StepIndicator } from './StepIndicator';

export type WizardStep = 'welcome' | 'security' | 'completion';

export interface WizardState {
  username: string;
  password: string;
  apiToken: string;
}

interface SetupWizardProps {
  onComplete: (nextPath?: string) => void;
  bootstrapToken?: string;
  isUnlocked?: boolean;
  setupWindowOpen?: boolean;
}

export const SetupWizard: Component<SetupWizardProps> = (props) => {
  const defaultWizardState: WizardState = {
    username: 'admin',
    password: '',
    apiToken: '',
  };
  const [directSetupActive, setDirectSetupActive] = createSignal(Boolean(props.setupWindowOpen));
  const [setupWindowExpired, setSetupWindowExpired] = createSignal(false);
  const [currentStep, setCurrentStep] = createSignal<WizardStep>(
    props.setupWindowOpen ? 'security' : 'welcome',
  );
  const [wizardState, setWizardState] = createSignal<WizardState>(defaultWizardState);
  const [bootstrapToken, setBootstrapToken] = createSignal(props.bootstrapToken || '');
  const [isUnlocked, setIsUnlocked] = createSignal(props.isUnlocked || false);

  const steps = createMemo<WizardStep[]>(() =>
    directSetupActive() ? ['security', 'completion'] : ['welcome', 'security', 'completion'],
  );

  const currentStepIndex = () => {
    const idx = steps().indexOf(currentStep());
    return idx >= 0 ? idx : 0;
  };

  createEffect(() => {
    currentStep();
    if (typeof window !== 'undefined' && typeof window.scrollTo === 'function') {
      window.scrollTo({ top: 0, behavior: 'auto' });
    }
  });

  const nextStep = () => {
    const activeSteps = steps();
    const idx = currentStepIndex();
    if (idx < activeSteps.length - 1) {
      setCurrentStep(activeSteps[idx + 1]);
    }
  };

  const prevStep = () => {
    const activeSteps = steps();
    const idx = currentStepIndex();
    if (idx > 0) {
      setCurrentStep(activeSteps[idx - 1]);
    }
  };

  const handleRequireUnlock = () => {
    setDirectSetupActive(false);
    setSetupWindowExpired(true);
    setIsUnlocked(false);
    setCurrentStep('welcome');
  };

  const updateState = (updates: Partial<WizardState>) => {
    setWizardState((prev) => ({ ...prev, ...updates }));
  };

  const handleComplete = (nextPath?: string) => {
    props.onComplete(nextPath);
  };

  const finishSecurityStep = () => {
    setCurrentStep('completion');
  };

  const stepLabels = createMemo(() =>
    directSetupActive()
      ? [t('setup.step.security'), t('setup.step.firstSource')]
      : [t('setup.step.unlockServer'), t('setup.step.security'), t('setup.step.firstSource')],
  );

  return (
    <div class="min-h-screen flex flex-col" role="main" aria-label={t('setup.wizard.ariaLabel')}>
      {/* Background decoration */}
      <div class="fixed inset-0 overflow-hidden pointer-events-none" aria-hidden="true"></div>

      <div
        class="relative z-10 pt-5 pb-1 px-4"
        role="navigation"
        aria-label={t('setup.progress.ariaLabel')}
      >
        <StepIndicator steps={stepLabels()} currentStep={currentStepIndex()} />
      </div>

      {/* Main content */}
      <div class="flex-1 flex items-start sm:items-center justify-center px-4 py-4 relative z-10">
        <div class="w-full max-w-2xl" role="region" aria-live="polite">
          <Show when={currentStep() === 'welcome'}>
            <WelcomeStep
              onNext={nextStep}
              bootstrapToken={bootstrapToken()}
              setBootstrapToken={setBootstrapToken}
              isUnlocked={isUnlocked()}
              setIsUnlocked={setIsUnlocked}
              windowExpired={setupWindowExpired()}
            />
          </Show>

          <Show when={currentStep() === 'security'}>
            <SecurityStep
              state={wizardState()}
              updateState={updateState}
              bootstrapToken={bootstrapToken()}
              showBack={!directSetupActive()}
              showWelcomeHeader={directSetupActive()}
              onRequireUnlock={handleRequireUnlock}
              onComplete={finishSecurityStep}
              onBack={prevStep}
            />
          </Show>

          <Show when={currentStep() === 'completion'}>
            <SetupCompletionPanel state={wizardState()} onComplete={handleComplete} />
          </Show>
        </div>
      </div>
    </div>
  );
};
