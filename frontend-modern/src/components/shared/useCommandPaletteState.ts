import { useLocation, useNavigate } from '@solidjs/router';
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
import { getAssistantPageContext } from '@/utils/assistantPageContext';
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
  const location = useLocation();
  const [query, setQuery] = createSignal('');
  const [inputRef, setInputRef] = createSignal<HTMLInputElement>();
  const [selectedIndex, setSelectedIndex] = createSignal(0);
  const assistantPageContext = createMemo(() => getAssistantPageContext(location.pathname));

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
      assistantOpenPresentation: {
        label: assistantPageContext().commandLabel,
        description: assistantPageContext().commandDescription,
      },
      assistantActions: {
        open: () =>
          runAfterPaletteSelection(() => aiChatStore.open(assistantPageContext().context)),
        help: () => requestAssistantCommand('help'),
        newSession: () => requestAssistantCommand('new'),
        sessions: () => requestAssistantCommand('sessions'),
        models: () => requestAssistantCommand('models'),
        providers: () => requestAssistantCommand('providers'),
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

  const moveSelection = (offset: number) => {
    const total = filteredCommands().length;
    if (total <= 0) return;
    setSelectedIndex((index) => (((index + offset) % total) + total) % total);
  };

  const handleInputKeyDown = (event: KeyboardEvent) => {
    const total = filteredCommands().length;
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        moveSelection(1);
        return;
      case 'ArrowUp':
        event.preventDefault();
        moveSelection(-1);
        return;
      case 'PageDown':
        event.preventDefault();
        moveSelection(5);
        return;
      case 'PageUp':
        event.preventDefault();
        moveSelection(-5);
        return;
      case 'Home':
        event.preventDefault();
        setSelectedIndex(0);
        return;
      case 'End':
        event.preventDefault();
        setSelectedIndex(Math.max(0, total - 1));
        return;
      case 'Enter': {
        const selected = filteredCommands()[selectedIndex()] ?? filteredCommands()[0];
        if (!selected) return;
        event.preventDefault();
        handleSelect(selected);
        return;
      }
    }
  };

  createEffect(() => {
    query();
    setSelectedIndex(0);
  });

  createEffect(() => {
    const total = filteredCommands().length;
    setSelectedIndex((index) => (total > 0 ? Math.min(index, total - 1) : 0));
  });

  createEffect(() => {
    if (props.isOpen) {
      setQuery('');
      setSelectedIndex(0);
      queueMicrotask(() => inputRef()?.focus());
      return;
    }

    setQuery('');
    setSelectedIndex(0);
  });

  return {
    filteredCommands,
    handleInputKeyDown,
    handleSelect,
    query,
    selectedIndex,
    setSelectedIndex,
    setInputRef,
    setQuery,
  };
}
