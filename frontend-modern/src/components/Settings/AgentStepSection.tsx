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
    <section class="space-y-3">
      <header class="flex flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between">
        <div>
          <p class="text-xs font-semibold uppercase tracking-wide text-blue-600 dark:text-blue-300">
            {props.step}
          </p>
          <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">
            {props.title}
          </h3>
        </div>
        <Show when={props.description}>
          <p class="text-sm text-gray-600 dark:text-gray-400 sm:text-right">
            {props.description}
          </p>
        </Show>
      </header>
      <div>{props.children}</div>
    </section>
  );
};

export default AgentStepSection;
