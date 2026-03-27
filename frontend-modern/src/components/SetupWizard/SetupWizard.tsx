import { Component, createSignal, Show } from 'solid-js';
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
}

export const SetupWizard: Component<SetupWizardProps> = (props) => {
  const defaultWizardState: WizardState = {
    username: 'admin',
    password: '',
    apiToken: '',
  };
  const [currentStep, setCurrentStep] = createSignal<WizardStep>('welcome');
  const [wizardState, setWizardState] = createSignal<WizardState>(defaultWizardState);
  const [bootstrapToken, setBootstrapToken] = createSignal(props.bootstrapToken || '');
  const [isUnlocked, setIsUnlocked] = createSignal(props.isUnlocked || false);

  const steps: WizardStep[] = ['welcome', 'security', 'completion'];

  const currentStepIndex = () => steps.indexOf(currentStep());

  const nextStep = () => {
    const idx = currentStepIndex();
    if (idx < steps.length - 1) {
      setCurrentStep(steps[idx + 1]);
    }
  };

  const prevStep = () => {
    const idx = currentStepIndex();
    if (idx > 0) {
      setCurrentStep(steps[idx - 1]);
    }
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

  const stepLabels = ['Unlock', 'Security', 'Install'];

  return (
    <div class="min-h-screen bg-base flex flex-col" role="main" aria-label="Pulse Setup Wizard">
      {/* Background decoration */}
      <div class="fixed inset-0 overflow-hidden pointer-events-none" aria-hidden="true"></div>

      <div class="relative z-10 pt-8 px-4" role="navigation" aria-label="Setup progress">
        <StepIndicator steps={stepLabels} currentStep={currentStepIndex()} />
      </div>

      {/* Main content */}
      <div class="flex-1 flex items-center justify-center p-4 relative z-10">
        <div class="w-full max-w-2xl" role="region" aria-live="polite">
          <Show when={currentStep() === 'welcome'}>
            <WelcomeStep
              onNext={nextStep}
              bootstrapToken={bootstrapToken()}
              setBootstrapToken={setBootstrapToken}
              isUnlocked={isUnlocked()}
              setIsUnlocked={setIsUnlocked}
            />
          </Show>

          <Show when={currentStep() === 'security'}>
            <SecurityStep
              state={wizardState()}
              updateState={updateState}
              bootstrapToken={bootstrapToken()}
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
