import { type Component } from 'solid-js';
import type { ExploreStatus } from './types';
import { getAIExploreStatusPresentation } from '@/utils/aiExplorePresentation';

interface ExploreStatusBlockProps {
  status: ExploreStatus;
}

export const ExploreStatusBlock: Component<ExploreStatusBlockProps> = (props) => {
  const presentation = () => getAIExploreStatusPresentation(props.status.phase);

  return (
    <div class={`my-2 rounded-md border px-3 py-2 text-xs ${presentation().classes}`}>
      <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
        <span class="font-semibold tracking-wide uppercase">{presentation().label}</span>
        {props.status.model && <span class="font-mono opacity-80">{props.status.model}</span>}
        {props.status.outcome && (
          <span class="font-mono opacity-75">outcome={props.status.outcome}</span>
        )}
      </div>
      <p class="mt-1 leading-relaxed">{props.status.message}</p>
    </div>
  );
};
