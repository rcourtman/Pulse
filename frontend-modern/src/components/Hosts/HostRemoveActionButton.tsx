import { Component } from 'solid-js';

interface HostRemoveActionButtonProps {
  onClick: () => void;
  disabled?: boolean;
  loading?: boolean;
  class?: string;
}

export const HostRemoveActionButton: Component<HostRemoveActionButtonProps> = (props) => {
  return (
    <button
      type="button"
      onClick={props.onClick}
      disabled={props.disabled}
      class={`flex w-full items-center gap-2 rounded-md px-2 py-2 text-left text-sm text-red-600 transition-colors hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-60 dark:text-red-300 dark:hover:bg-red-900/20 ${props.class ?? ''}`.trim()}
    >
      {props.loading ? (
        <svg class="h-3.5 w-3.5 animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke-width="3" />
          <path class="opacity-75" stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M4 12a8 8 0 018-8" />
        </svg>
      ) : (
        <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6M9 7V4a1 1 0 011-1h4a1 1 0 011 1v3m-7 0h8" />
        </svg>
      )}
      <span>{props.loading ? 'Removing...' : 'Remove host from Pulse'}</span>
    </button>
  );
};
