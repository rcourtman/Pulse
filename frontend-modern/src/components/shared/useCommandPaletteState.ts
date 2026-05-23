import { useNavigate } from '@solidjs/router';
import { createEffect, createMemo, createSignal } from 'solid-js';
import {
  buildAgentsPath,
  buildDockerPath,
  buildKubernetesPath,
  buildProxmoxPath,
  buildTrueNASPath,
  buildVmwarePath,
} from '@/routing/resourceLinks';
import {
  buildCommandPaletteCommands,
  filterCommandPaletteCommands,
  type CommandPaletteModalCommand,
  type CommandPaletteModalProps,
} from './commandPaletteModel';

export type { CommandPaletteModalCommand, CommandPaletteModalProps } from './commandPaletteModel';

export function useCommandPaletteState(props: CommandPaletteModalProps) {
  const navigate = useNavigate();
  const [query, setQuery] = createSignal('');
  const [inputRef, setInputRef] = createSignal<HTMLInputElement>();

  const commands = createMemo(() =>
    buildCommandPaletteCommands({
      paths: {
        agentsPath: buildAgentsPath(),
        proxmoxPath: buildProxmoxPath(),
        dockerPath: buildDockerPath(),
        kubernetesPath: buildKubernetesPath(),
        kubernetesPodsPath: buildKubernetesPath('pods'),
        trueNasPath: buildTrueNASPath(),
        vmwarePath: buildVmwarePath(),
        vmwareNetworksPath: buildVmwarePath('networks'),
      },
      infrastructureVisibility: props.infrastructureVisibility(),
      navigate,
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

export type CommandPaletteState = ReturnType<typeof useCommandPaletteState>;
