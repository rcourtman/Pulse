import { Component } from 'solid-js';

interface StepIndicatorProps {
  steps: string[];
  currentStep: number;
}

export const StepIndicator: Component<StepIndicatorProps> = (props) => {
  return (
    <div class="flex items-center justify-center gap-2" role="list">
      {props.steps.map((step, index) => {
        const state =
          index < props.currentStep
            ? ', completed'
            : index === props.currentStep
              ? ', current'
              : '';
        return (
          <div class="flex items-center" role="listitem">
            <div
              class={`flex items-center gap-2 px-3 py-1.5 rounded-full text-sm font-medium transition-all ${
                index < props.currentStep
                  ? 'bg-emerald-600 text-white border border-emerald-500'
                  : index === props.currentStep
                    ? 'bg-blue-600 text-white border border-blue-500'
                    : 'bg-surface border border-border text-muted'
              }`}
              aria-current={index === props.currentStep ? 'step' : undefined}
              aria-label={`Step ${index + 1}: ${step}${state}`}
            >
              <span
                class={`w-5 h-5 flex items-center justify-center rounded-full text-xs ${
                  index < props.currentStep
                    ? 'bg-emerald-700 text-white'
                    : index === props.currentStep
                      ? 'bg-blue-700 text-white'
                      : 'bg-surface-alt border border-border text-muted'
                }`}
                aria-hidden="true"
              >
                {index < props.currentStep ? '✓' : index + 1}
              </span>
              <span class="hidden sm:inline" aria-hidden="true">
                {step}
              </span>
            </div>
            {index < props.steps.length - 1 && (
              <div
                class={`w-8 h-0.5 mx-1 ${index < props.currentStep ? 'bg-emerald-500' : 'bg-border'}`}
                aria-hidden="true"
              />
            )}
          </div>
        );
      })}
    </div>
  );
};
