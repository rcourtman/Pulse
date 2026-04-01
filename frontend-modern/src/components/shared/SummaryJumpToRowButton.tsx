import type { Component } from 'solid-js';

interface SummaryJumpToRowButtonProps {
  onClick: () => void;
}

export const SummaryJumpToRowButton: Component<SummaryJumpToRowButtonProps> = (props) => {
  return (
    <button
      type="button"
      onClick={props.onClick}
      class="inline-flex items-center gap-1 rounded-full border border-border-subtle bg-surface-alt/70 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted transition-colors hover:bg-surface-hover hover:text-base-content"
    >
      <span>Jump to row</span>
      <svg class="h-3 w-3" viewBox="0 0 16 16" fill="none" stroke="currentColor">
        <path
          d="M8 3v10m0 0-3-3m3 3 3-3"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
    </button>
  );
};
