import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import commandPaletteModalSource from '@/components/shared/CommandPaletteModal.tsx?raw';
import commandPaletteModelSource from '@/components/shared/commandPaletteModel.ts?raw';
import commandPaletteStateSource from '@/components/shared/useCommandPaletteState.ts?raw';

const navigateMock = vi.fn();

vi.mock('@solidjs/router', () => ({
  useNavigate: () => navigateMock,
}));

vi.mock('@/components/shared/Dialog', () => ({
  Dialog: (props: { isOpen: boolean; children: JSX.Element }) =>
    props.isOpen ? <div>{props.children}</div> : null,
}));

import { CommandPaletteModal } from '@/components/shared/CommandPaletteModal';

describe('CommandPaletteModal', () => {
  afterEach(() => {
    cleanup();
    navigateMock.mockReset();
  });

  it('keeps the command palette on shell, runtime, and model owners', () => {
    expect(commandPaletteModalSource).toContain('useCommandPaletteState');
    expect(commandPaletteModalSource).not.toContain('useNavigate');
    expect(commandPaletteModalSource).not.toContain('createSignal');
    expect(commandPaletteModalSource).not.toContain('buildInfrastructurePath');

    expect(commandPaletteStateSource).toContain('useNavigate');
    expect(commandPaletteStateSource).toContain('createSignal');
    expect(commandPaletteStateSource).toContain('buildInfrastructurePath');
    expect(commandPaletteStateSource).toContain('export function useCommandPaletteState');

    expect(commandPaletteModelSource).toContain('buildCommandPaletteCommands');
    expect(commandPaletteModelSource).toContain('normalizeCommandPaletteQuery');
    expect(commandPaletteModelSource).toContain('filterCommandPaletteCommands');
  });

  it('renders the dedicated pod workloads command with a canonical path', () => {
    render(() => <CommandPaletteModal isOpen={true} onClose={vi.fn()} />);

    expect(screen.getByText('Go to Kubernetes Pods')).toBeInTheDocument();
    expect(screen.getByText('/workloads?type=pod')).toBeInTheDocument();
  });

  it('navigates to the canonical pod workloads path', async () => {
    const onClose = vi.fn();
    render(() => <CommandPaletteModal isOpen={true} onClose={onClose} />);

    await fireEvent.click(screen.getByText('Go to Kubernetes Pods'));

    expect(navigateMock).toHaveBeenCalledWith('/workloads?type=pod');
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('uses the shared search input and keeps Enter selection behavior', async () => {
    const onClose = vi.fn();
    render(() => <CommandPaletteModal isOpen={true} onClose={onClose} />);

    const input = screen.getByPlaceholderText('Type a command or search...');
    await fireEvent.input(input, { target: { value: 'type=pod' } });
    await fireEvent.keyDown(input, { key: 'Enter' });

    expect(navigateMock).toHaveBeenCalledWith('/workloads?type=pod');
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
