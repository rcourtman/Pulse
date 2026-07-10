import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';

import NotFound from '@/pages/NotFound';

const navigate = vi.fn();

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ pathname: '/missing', search: '?from=test' }),
  useNavigate: () => navigate,
}));

describe('NotFound', () => {
  it('offers a touch-sized route back to the workspace', () => {
    render(() => <NotFound />);

    expect(screen.getByText('No route matched /missing?from=test.')).toBeInTheDocument();
    const action = screen.getByRole('button', { name: 'Go to workspace' });
    expect(action).toHaveClass('min-h-10');

    action.click();
    expect(navigate).toHaveBeenCalledWith('/');
  });
});
