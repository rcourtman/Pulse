import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { Docker } from '../Docker';

const mocks = vi.hoisted(() => ({
  pathname: '/docker/overview',
}));

vi.mock('@solidjs/router', () => ({
  Navigate: (props: { href: string }) => <div data-testid="navigate" data-href={props.href} />,
  useLocation: () => ({
    get pathname() {
      return mocks.pathname;
    },
  }),
}));

vi.mock('@/features/docker/DockerPageSurface', () => ({
  DockerPageSurface: () => <div data-testid="docker-page-surface" />,
}));

afterEach(() => {
  cleanup();
  mocks.pathname = '/docker/overview';
});

describe('Docker page route ownership', () => {
  it('renders the canonical Docker surface for current routes', () => {
    render(() => <Docker />);

    expect(screen.getByTestId('docker-page-surface')).toBeInTheDocument();
    expect(screen.queryByTestId('navigate')).not.toBeInTheDocument();
  });

  it('redirects the retired workload route to the canonical overview', () => {
    mocks.pathname = '/docker/workloads/';

    render(() => <Docker />);

    expect(screen.getByTestId('navigate')).toHaveAttribute('data-href', '/docker/overview');
    expect(screen.queryByTestId('docker-page-surface')).not.toBeInTheDocument();
  });
});
