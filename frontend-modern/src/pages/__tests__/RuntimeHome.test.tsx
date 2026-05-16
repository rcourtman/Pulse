import { render, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import RuntimeHome from '@/pages/RuntimeHome';

const navigateSpy = vi.hoisted(() => vi.fn());

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useNavigate: () => navigateSpy,
  };
});

describe('RuntimeHome', () => {
  beforeEach(() => {
    navigateSpy.mockReset();
  });

  it('routes authenticated runtimes straight to the Proxmox platform page', async () => {
    render(() => <RuntimeHome />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/proxmox/overview', { replace: true });
    });
  });
});
