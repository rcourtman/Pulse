/**
 * ConsoleButton: A compact button to open a console session for a VM.
 * Can be placed in guest drawers, guest rows, or toolbars.
 */
import { Component, Show, createSignal } from 'solid-js';
import { ConsoleModal } from './ConsoleModal';

interface ConsoleButtonProps {
  vmId: string;
  vmName: string;
  nodeId: string;
  providerId: string;
  consoleTypes?: string[];
  compact?: boolean; // Icon-only mode for table rows
}

export const ConsoleButton: Component<ConsoleButtonProps> = (props) => {
  const [showConsole, setShowConsole] = createSignal(false);

  const types = () => props.consoleTypes || ['vnc'];
  const hasConsole = () => types().length > 0;

  return (
    <>
      <Show when={hasConsole()}>
        <button
          class="inline-flex items-center gap-1.5 text-xs font-medium text-blue-400 hover:text-blue-300 transition-colors"
          classList={{
            'px-2 py-1 rounded border border-gray-600 hover:border-blue-500 bg-gray-700/50': !props.compact,
            'p-1 rounded hover:bg-gray-700': props.compact,
          }}
          onClick={(e) => {
            e.stopPropagation();
            setShowConsole(true);
          }}
          title={`Open console (${types().join(', ')})`}
        >
          {/* Terminal icon */}
          <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
          </svg>
          <Show when={!props.compact}>
            <span>Console</span>
          </Show>
        </button>
      </Show>

      <Show when={showConsole()}>
        <ConsoleModal
          vmId={props.vmId}
          vmName={props.vmName}
          nodeId={props.nodeId}
          providerId={props.providerId}
          consoleTypes={types()}
          onClose={() => setShowConsole(false)}
        />
      </Show>
    </>
  );
};
