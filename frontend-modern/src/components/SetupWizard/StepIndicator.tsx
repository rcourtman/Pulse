import { Component } from 'solid-js';

interface StepIndicatorProps {
    steps: string[];
    currentStep: number;
}

export const StepIndicator: Component<StepIndicatorProps> = (props) => {
    return (
        <div class="flex items-center justify-center gap-2">
            {props.steps.map((step, index) => (
                <div class="flex items-center">
                    <div class={`flex items-center gap-2 px-3 py-1.5 rounded-full text-sm font-medium transition-all ${index < props.currentStep
                            ? 'bg-emerald-600 text-white border border-emerald-500'
                            : index === props.currentStep
                                ? 'bg-blue-600 text-white border border-blue-500'
                                : 'bg-surface border border-border text-muted'
                        }`}>
                        <span class={`w-5 h-5 flex items-center justify-center rounded-full text-xs ${index < props.currentStep
                                ? 'bg-emerald-700 text-white'
                                : index === props.currentStep
                                    ? 'bg-blue-700 text-white'
                                    : 'bg-surface-alt border border-border text-muted'
                            }`}>
                            {index < props.currentStep ? '✓' : index + 1}
                        </span>
                        <span class="hidden sm:inline">{step}</span>
                    </div>
                    {index < props.steps.length - 1 && (
                        <div class={`w-8 h-0.5 mx-1 ${index < props.currentStep ? 'bg-emerald-500' : 'bg-border'
                            }`} />
                    )}
                </div>
            ))}
        </div>
    );
};
