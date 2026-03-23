import { useNavigate } from '@solidjs/router';
import { createEffect, createMemo, createSignal } from 'solid-js';
import {
  buildRecoveryPath,
  buildInfrastructurePath,
  buildStoragePath,
  buildWorkloadsPath,
} from '@/routing/resourceLinks';
import {
  buildCommandPaletteCommands,
  filterCommandPaletteCommands,
  type CommandPaletteModalCommand,
  type CommandPaletteModalProps,
} from './commandPaletteModel';

export type {
  CommandPaletteModalCommand,
  CommandPaletteModalProps,
} from './commandPaletteModel';

export function useCommandPaletteState(props: CommandPaletteModalProps) {
  const navigate = useNavigate();
  const [query, setQuery] = createSignal('');
  const [inputRef, setInputRef] = createSignal<HTMLInputElement>();

  const commands = createMemo(() =>
    buildCommandPaletteCommands({
      paths: {
        infrastructurePath: buildInfrastructurePath(),
        workloadsPath: buildWorkloadsPath(),
        podWorkloadsPath: buildWorkloadsPath({ type: 'pod' }),
        storagePath: buildStoragePath(),
        recoveryPath: buildRecoveryPath(),
      },
      navigate,
    }),
  );

  const filteredCommands = createMemo(() =>
    filterCommandPaletteCommands(commands(), query()),
  );

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
