import type { Component, JSX } from 'solid-js';
import { Show } from 'solid-js';

interface AgentStepSectionProps {
  step: string;
  title: string;
  description?: string;
  children: JSX.Element;
}

export const AgentStepSection: Component<AgentStepSectionProps> = (props) => {
  return (
    <section class="space-y-5">
      <header class="space-y-2">
        <div class="flex items-center gap-2">
          <span class="inline-flex items-center justify-center w-7 h-7 rounded-full bg-blue-100 dark:bg-blue-900/30 text-xs font-bold text-blue-700 dark:text-blue-300">
            {props.step.replace('Step ', '')}
          </span>
          <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
            {props.title}
          </h3>
        </div>
        <Show when={props.description}>
          <p class="text-sm text-gray-600 dark:text-gray-400 leading-relaxed ml-9">
            {props.description}
          </p>
        </Show>
      </header>
      <div class="ml-9">{props.children}</div>
    </section>
  );
};

export default AgentStepSection;
