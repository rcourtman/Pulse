import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';

const navigateMock = vi.fn();

vi.mock('@solidjs/router', () => ({
  useNavigate: () => navigateMock,
}));

vi.mock('@/components/shared/Dialog', () => ({
  Dialog: (props: { isOpen: boolean; children: unknown }) =>
    props.isOpen ? <div>{props.children}</div> : null,
}));

import { CommandPaletteModal } from '@/components/shared/CommandPaletteModal';

describe('CommandPaletteModal', () => {
  afterEach(() => {
    cleanup();
    navigateMock.mockReset();
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
});
