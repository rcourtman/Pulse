import { useNavigate } from '@solidjs/router';
import { createEffect, createMemo, createSignal } from 'solid-js';
import {
  buildDockerPath,
  buildKubernetesPath,
  buildProxmoxPath,
  buildStandalonePath,
  buildTrueNASPath,
  buildVmwarePath,
} from '@/routing/resourceLinks';
import { aiChatStore, type AIChatCommandRequestAction } from '@/stores/aiChat';
import {
  buildCommandPaletteCommands,
  filterCommandPaletteCommands,
  type CommandPaletteModalCommand,
  type CommandPaletteModalProps,
} from './commandPaletteModel';

export type { CommandPaletteModalCommand, CommandPaletteModalProps } from './commandPaletteModel';

const runAfterPaletteSelection = (action: () => void) => {
  queueMicrotask(action);
};

export function useCommandPaletteState(props: CommandPaletteModalProps) {
  const navigate = useNavigate();
  const [query, setQuery] = createSignal('');
  const [inputRef, setInputRef] = createSignal<HTMLInputElement>();

  const requestAssistantCommand = (action: AIChatCommandRequestAction) => {
    runAfterPaletteSelection(() => aiChatStore.requestCommand(action));
  };

  const commands = createMemo(() =>
    buildCommandPaletteCommands({
      paths: {
        standalonePath: buildStandalonePath(),
        proxmoxPath: buildProxmoxPath(),
        dockerPath: buildDockerPath(),
        kubernetesPath: buildKubernetesPath(),
        kubernetesWorkloadsPath: buildKubernetesPath('workloads'),
        trueNasPath: buildTrueNASPath(),
        vmwarePath: buildVmwarePath(),
        vmwareNetworksPath: buildVmwarePath('networks'),
      },
      platformVisibility: props.platformVisibility(),
      navigate,
      assistantActions: {
        open: () => runAfterPaletteSelection(() => aiChatStore.open()),
        newSession: () => requestAssistantCommand('new'),
        sessions: () => requestAssistantCommand('sessions'),
        models: () => requestAssistantCommand('models'),
        status: () => requestAssistantCommand('status'),
        undo: () => requestAssistantCommand('undo'),
        redo: () => requestAssistantCommand('redo'),
      },
    }),
  );

  const filteredCommands = createMemo(() => filterCommandPaletteCommands(commands(), query()));

  const handleSelect = (command: CommandPaletteModalCommand) => {
    command.action();
    props.onClose();
  };

  const handleInputKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'Enter') return;
    const first = filteredCommands()[0];
    if (!first) return;
    event.preventDefault();
    handleSelect(first);
  };

  createEffect(() => {
    if (props.isOpen) {
      setQuery('');
      queueMicrotask(() => inputRef()?.focus());
      return;
    }

    setQuery('');
  });

  return {
    filteredCommands,
    handleInputKeyDown,
    handleSelect,
    query,
    setInputRef,
    setQuery,
  };
}
