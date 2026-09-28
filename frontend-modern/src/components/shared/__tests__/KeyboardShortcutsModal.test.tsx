import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import { createEmptyPlatformNavigationVisibility } from '@/features/platformNavigation/platformNavigationModel';

vi.mock('@/components/shared/Dialog', () => ({
  Dialog: (props: { isOpen: boolean; children: JSX.Element }) =>
    props.isOpen ? <div>{props.children}</div> : null,
}));

import { KeyboardShortcutsModal } from '@/components/shared/KeyboardShortcutsModal';

describe('KeyboardShortcutsModal', () => {
  afterEach(() => {
    cleanup();
  });

  const renderModal = (patrolVisible: boolean) =>
    render(() => (
      <KeyboardShortcutsModal
        isOpen={true}
        onClose={vi.fn()}
        platformVisibility={createEmptyPlatformNavigationVisibility}
        patrolVisible={() => patrolVisible}
      />
    ));

  it('lists the Patrol shortcut while Patrol navigation is visible', () => {
    renderModal(true);

    expect(screen.getByText('Go to Patrol')).toBeInTheDocument();
    expect(screen.getByText('Go to Alerts')).toBeInTheDocument();
  });

  it('drops the Patrol shortcut while AI is off (#905)', () => {
    renderModal(false);

    expect(screen.queryByText('Go to Patrol')).not.toBeInTheDocument();
    expect(screen.getByText('Go to Alerts')).toBeInTheDocument();
    expect(screen.getByText('Go to Settings')).toBeInTheDocument();
  });
});
