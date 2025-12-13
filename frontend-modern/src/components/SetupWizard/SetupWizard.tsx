import { Component, createSignal, Show, onMount } from 'solid-js';
import { WelcomeStep } from './steps/WelcomeStep';
import { SecurityStep } from './steps/SecurityStep';
import { ConnectStep } from './steps/ConnectStep';
import { FeaturesStep } from './steps/FeaturesStep';
import { CompleteStep } from './steps/CompleteStep';
import { StepIndicator } from './StepIndicator';

export type WizardStep = 'welcome' | 'security' | 'connect' | 'features' | 'complete';

export interface WizardState {
    // Security
    username: string;
    password: string;
    apiToken: string;
    // Node
    nodeAdded: boolean;
    nodeName: string;
    // Features
    aiEnabled: boolean;
    autoUpdatesEnabled: boolean;
}

interface SetupWizardProps {
    onComplete: () => void;
    bootstrapToken?: string;
    isUnlocked?: boolean;
}

export const SetupWizard: Component<SetupWizardProps> = (props) => {
    const [currentStep, setCurrentStep] = createSignal<WizardStep>('welcome');
    const [wizardState, setWizardState] = createSignal<WizardState>({
        username: 'admin',
        password: '',
        apiToken: '',
        nodeAdded: false,
        nodeName: '',
        aiEnabled: false,
        autoUpdatesEnabled: true,
    });
    const [bootstrapToken, setBootstrapToken] = createSignal(props.bootstrapToken || '');
    const [isUnlocked, setIsUnlocked] = createSignal(props.isUnlocked || false);

    const steps: WizardStep[] = ['welcome', 'security', 'connect', 'features', 'complete'];

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
        setWizardState(prev => ({ ...prev, ...updates }));
    };

    const skipToComplete = () => {
        setCurrentStep('complete');
    };

    return (
        <div class="min-h-screen bg-gradient-to-br from-slate-900 via-blue-900 to-indigo-900 flex flex-col">
            {/* Background decoration */}
            <div class="fixed inset-0 overflow-hidden pointer-events-none">
                <div class="absolute -top-40 -right-40 w-80 h-80 bg-blue-500/20 rounded-full blur-3xl" />
                <div class="absolute top-1/2 -left-40 w-80 h-80 bg-indigo-500/20 rounded-full blur-3xl" />
                <div class="absolute -bottom-40 right-1/3 w-80 h-80 bg-purple-500/20 rounded-full blur-3xl" />
            </div>

            {/* Step indicator - only show after welcome */}
            <Show when={currentStep() !== 'welcome' && currentStep() !== 'complete'}>
                <div class="relative z-10 pt-8 px-4">
                    <StepIndicator
                        steps={['Security', 'Connect', 'Features']}
                        currentStep={currentStepIndex() - 1}
                    />
                </div>
            </Show>

            {/* Main content */}
            <div class="flex-1 flex items-center justify-center p-4 relative z-10">
                <div class="w-full max-w-2xl">
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
                            onNext={nextStep}
                            onBack={prevStep}
                        />
                    </Show>

                    <Show when={currentStep() === 'connect'}>
                        <ConnectStep
                            state={wizardState()}
                            updateState={updateState}
                            onNext={nextStep}
                            onBack={prevStep}
                            onSkip={skipToComplete}
                        />
                    </Show>

                    <Show when={currentStep() === 'features'}>
                        <FeaturesStep
                            state={wizardState()}
                            updateState={updateState}
                            onNext={nextStep}
                            onBack={prevStep}
                        />
                    </Show>

                    <Show when={currentStep() === 'complete'}>
                        <CompleteStep
                            state={wizardState()}
                            onComplete={props.onComplete}
                        />
                    </Show>
                </div>
            </div>
        </div>
    );
};
