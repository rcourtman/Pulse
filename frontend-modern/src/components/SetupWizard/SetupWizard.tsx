import { Component, createSignal, Show } from 'solid-js';
import { WelcomeStep } from './steps/WelcomeStep';
import { SecurityStep } from './steps/SecurityStep';
import { CompleteStep } from './steps/CompleteStep';
import { StepIndicator } from './StepIndicator';
import { STORAGE_KEYS } from '@/utils/localStorage';

export type WizardStep = 'welcome' | 'security' | 'complete';

export interface WizardState {
    // Security
    username: string;
    password: string;
    apiToken: string;
    // Node (for display in complete step)
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
    const defaultWizardState: WizardState = {
        username: 'admin',
        password: '',
        apiToken: '',
        nodeAdded: false,
        nodeName: '',
        aiEnabled: false,
        autoUpdatesEnabled: true,
    };

    const loadStoredCredentials = (): WizardState | null => {
        if (typeof window === 'undefined') return null;
        try {
            const raw = sessionStorage.getItem(STORAGE_KEYS.SETUP_CREDENTIALS);
            if (!raw) return null;
            const parsed = JSON.parse(raw) as Partial<WizardState>;
            if (!parsed.username || !parsed.password || !parsed.apiToken) {
                sessionStorage.removeItem(STORAGE_KEYS.SETUP_CREDENTIALS);
                return null;
            }
            return {
                ...defaultWizardState,
                username: parsed.username,
                password: parsed.password,
                apiToken: parsed.apiToken,
            };
        } catch (_err) {
            try {
                sessionStorage.removeItem(STORAGE_KEYS.SETUP_CREDENTIALS);
            } catch (_removeErr) {
                // Ignore cleanup errors
            }
            return null;
        }
    };

    const storedCredentials = loadStoredCredentials();
    const [currentStep, setCurrentStep] = createSignal<WizardStep>(storedCredentials ? 'complete' : 'welcome');
    const [wizardState, setWizardState] = createSignal<WizardState>(storedCredentials ?? defaultWizardState);
    const [bootstrapToken, setBootstrapToken] = createSignal(props.bootstrapToken || '');
    const [isUnlocked, setIsUnlocked] = createSignal(props.isUnlocked || Boolean(storedCredentials));

    const steps: WizardStep[] = ['welcome', 'security', 'complete'];

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

    const clearStoredCredentials = () => {
        if (typeof window === 'undefined') return;
        try {
            sessionStorage.removeItem(STORAGE_KEYS.SETUP_CREDENTIALS);
        } catch (_err) {
            // Ignore storage errors
        }
    };

    const handleComplete = () => {
        clearStoredCredentials();
        props.onComplete();
    };

    return (
        <div
            class="min-h-screen bg-slate-900 flex flex-col"
            role="main"
            aria-label="Pulse Setup Wizard"
        >
            {/* Background decoration */}
            <div class="fixed inset-0 overflow-hidden pointer-events-none" aria-hidden="true">
                <div class="absolute -top-40 -right-40 w-80 h-80 bg-blue-500/20 rounded-full blur-3xl" />
                <div class="absolute top-1/2 -left-40 w-80 h-80 bg-indigo-500/20 rounded-full blur-3xl" />
                <div class="absolute -bottom-40 right-1/3 w-80 h-80 bg-purple-500/20 rounded-full blur-3xl" />
            </div>

            {/* Step indicator - only show during security step */}
            <Show when={currentStep() === 'security'}>
                <div class="relative z-10 pt-8 px-4" role="navigation" aria-label="Setup progress">
                    <StepIndicator
                        steps={['Welcome', 'Security', 'Done']}
                        currentStep={1}
                    />
                </div>
            </Show>

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
                            onNext={nextStep}
                            onBack={prevStep}
                        />
                    </Show>

                    <Show when={currentStep() === 'complete'}>
                        <CompleteStep
                            state={wizardState()}
                            onComplete={handleComplete}
                        />
                    </Show>
                </div>
            </div>
        </div>
    );
};
