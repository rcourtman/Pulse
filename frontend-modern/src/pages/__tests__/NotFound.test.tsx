import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';

import NotFound from '@/pages/NotFound';

const navigate = vi.fn();

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ pathname: '/missing', search: '?from=test' }),
  useNavigate: () => navigate,
}));

describe('NotFound', () => {
  it('renders a single PageHeader heading, matched path note, and workspace link', () => {
    render(() => <NotFound />);

    expect(screen.getAllByRole('heading')).toHaveLength(1);
    expect(screen.getByRole('heading', { level: 1, name: 'Page not found' })).toBeInTheDocument();
    expect(screen.getByText('No route matched /missing?from=test.')).toBeInTheDocument();
    const action = screen.getByRole('button', { name: 'Go to workspace' });
    expect(action).toHaveClass('min-h-11');

    action.click();
    expect(navigate).toHaveBeenCalledWith('/');
  });
});
