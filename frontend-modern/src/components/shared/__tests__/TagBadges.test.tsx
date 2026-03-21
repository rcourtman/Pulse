import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@solidjs/testing-library';

// ── Hoisted mocks ──────────────────────────────────────────────────────

const { darkModeMock, showTooltipMock, hideTooltipMock } = vi.hoisted(() => {
  const darkModeMock = vi.fn(() => false);
  const showTooltipMock = vi.fn();
  const hideTooltipMock = vi.fn();
  return { darkModeMock, showTooltipMock, hideTooltipMock };
});

// ── Module mocks ───────────────────────────────────────────────────────

vi.mock('@/App', () => ({
  useDarkMode: () => darkModeMock,
}));

vi.mock('@/components/shared/Tooltip', () => ({
  showTooltip: showTooltipMock,
  hideTooltip: hideTooltipMock,
}));

const getTagColorWithSpecialMock = vi.fn((_tag: string, isDark: boolean) => ({
  bg: isDark ? 'rgb(30, 30, 30)' : 'rgb(200, 200, 200)',
  text: isDark ? 'rgb(240, 240, 240)' : 'rgb(20, 20, 20)',
  border: isDark ? 'rgb(80, 80, 80)' : 'rgb(150, 150, 150)',
}));

vi.mock('@/utils/tagColors', () => ({
  getTagColorWithSpecial: (...args: unknown[]) =>
    getTagColorWithSpecialMock(...(args as [string, boolean])),
}));

import { TagBadges } from '../TagBadges';

// ── Helpers ────────────────────────────────────────────────────────────

/** Return all tag dot elements (the colored circles) scoped to the test container. */
function getTagDots(container?: HTMLElement) {
  return (container ?? document).querySelectorAll('.rounded-full');
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  // Reset dark mode to default (false) for isolation
  darkModeMock.mockReturnValue(false);
});

// ── Tests ──────────────────────────────────────────────────────────────

describe('TagBadges', () => {
  describe('rendering visibility', () => {
    it('renders nothing when tags is undefined', () => {
      const { container } = render(() => <TagBadges />);
      expect(container.innerHTML).toBe('');
    });

    it('renders nothing when tags is an empty array', () => {
      const { container } = render(() => <TagBadges tags={[]} />);
      expect(container.innerHTML).toBe('');
    });

    it('renders tag dots when tags are provided', () => {
      render(() => <TagBadges tags={['web', 'db']} />);
      const dots = getTagDots();
      expect(dots.length).toBe(2);
    });
  });

  describe('maxVisible logic', () => {
    it('defaults maxVisible to 3 — shows 3 dots for 3 tags', () => {
      render(() => <TagBadges tags={['a', 'b', 'c']} />);
      const dots = getTagDots();
      expect(dots.length).toBe(3);
    });

    it('shows 3 visible + overflow indicator for 5 tags (default maxVisible=3)', () => {
      render(() => <TagBadges tags={['a', 'b', 'c', 'd', 'e']} />);
      const dots = getTagDots();
      // 3 visible dots
      expect(dots.length).toBe(3);
      // +2 overflow text
      expect(screen.getByText('+2')).toBeInTheDocument();
    });

    it('shows all dots when exactly 4 tags — the 4th is shown as a dot (not +1)', () => {
      render(() => <TagBadges tags={['a', 'b', 'c', 'd']} />);
      const dots = getTagDots();
      // 3 visible + 1 "only one hidden" → all 4 shown as dots
      expect(dots.length).toBe(4);
      // No +1 indicator
      expect(screen.queryByText('+1')).not.toBeInTheDocument();
    });

    it('shows +X when there are 2+ hidden tags (5 tags → +2)', () => {
      render(() => <TagBadges tags={['a', 'b', 'c', 'd', 'e']} />);
      expect(screen.getByText('+2')).toBeInTheDocument();
    });

    it('maxVisible=0 means show all tags', () => {
      const tags = ['a', 'b', 'c', 'd', 'e', 'f'];
      render(() => <TagBadges tags={tags} maxVisible={0} />);
      const dots = getTagDots();
      expect(dots.length).toBe(6);
      expect(screen.queryByText(/\+\d/)).not.toBeInTheDocument();
    });

    it('maxVisible=2 with 5 tags shows 2 dots + overflow', () => {
      render(() => <TagBadges tags={['a', 'b', 'c', 'd', 'e']} maxVisible={2} />);
      const dots = getTagDots();
      expect(dots.length).toBe(2);
      expect(screen.getByText('+3')).toBeInTheDocument();
    });

    it('maxVisible=2 with 3 tags shows all 3 (one hidden → shown as dot)', () => {
      render(() => <TagBadges tags={['a', 'b', 'c']} maxVisible={2} />);
      const dots = getTagDots();
      // 2 visible + 1 hidden shown as dot
      expect(dots.length).toBe(3);
      expect(screen.queryByText('+1')).not.toBeInTheDocument();
    });

    it('maxVisible=5 with 3 tags shows all 3', () => {
      render(() => <TagBadges tags={['a', 'b', 'c']} maxVisible={5} />);
      const dots = getTagDots();
      expect(dots.length).toBe(3);
    });
  });

  describe('tag dot colors', () => {
    it('calls getTagColorWithSpecial with isDark=false in light mode', () => {
      darkModeMock.mockReturnValue(false);
      render(() => <TagBadges tags={['web']} />);
      expect(getTagColorWithSpecialMock).toHaveBeenCalledWith('web', false);
      const dot = getTagDots()[0] as HTMLElement;
      expect(dot.style.backgroundColor).toBe('rgb(200, 200, 200)');
    });

    it('calls getTagColorWithSpecial with isDark=true when isDarkMode prop is true', () => {
      render(() => <TagBadges tags={['web']} isDarkMode={true} />);
      expect(getTagColorWithSpecialMock).toHaveBeenCalledWith('web', true);
      const dot = getTagDots()[0] as HTMLElement;
      expect(dot.style.backgroundColor).toBe('rgb(30, 30, 30)');
    });

    it('uses dark mode signal when isDarkMode prop is not set', () => {
      darkModeMock.mockReturnValue(true);
      render(() => <TagBadges tags={['db']} />);
      expect(getTagColorWithSpecialMock).toHaveBeenCalledWith('db', true);
      const dot = getTagDots()[0] as HTMLElement;
      expect(dot.style.backgroundColor).toBe('rgb(30, 30, 30)');
    });

    it('isDarkMode=false overrides dark mode signal', () => {
      darkModeMock.mockReturnValue(true);
      render(() => <TagBadges tags={['web']} isDarkMode={false} />);
      expect(getTagColorWithSpecialMock).toHaveBeenCalledWith('web', false);
      const dot = getTagDots()[0] as HTMLElement;
      expect(dot.style.backgroundColor).toBe('rgb(200, 200, 200)');
    });
  });

  describe('activeSearch highlighting', () => {
    it('applies box-shadow when tag is in activeSearch', () => {
      render(() => <TagBadges tags={['web']} activeSearch="tags:web" />);
      const dot = getTagDots()[0] as HTMLElement;
      expect(dot.style.boxShadow).not.toBe('none');
    });

    it('does not apply box-shadow when tag is not in activeSearch', () => {
      render(() => <TagBadges tags={['web']} activeSearch="tags:db" />);
      const dot = getTagDots()[0] as HTMLElement;
      expect(dot.style.boxShadow).toBe('none');
    });

    it('uses light-mode shadow color when not dark', () => {
      darkModeMock.mockReturnValue(false);
      render(() => <TagBadges tags={['web']} activeSearch="tags:web" />);
      const dot = getTagDots()[0] as HTMLElement;
      expect(dot.style.boxShadow).toContain('rgba(0, 0, 0');
    });

    it('uses dark-mode shadow color when dark', () => {
      darkModeMock.mockReturnValue(true);
      render(() => <TagBadges tags={['web']} activeSearch="tags:web" />);
      const dot = getTagDots()[0] as HTMLElement;
      expect(dot.style.boxShadow).toContain('rgba(255, 255, 255');
    });

    it('substring match: "web" highlights when activeSearch contains "tags:webapp" (documents current behavior)', () => {
      // Documents that the component uses .includes() for matching, which
      // means substring matches occur. If exact-token matching is added later,
      // update this test to expect boxShadow === 'none'.
      render(() => <TagBadges tags={['web']} activeSearch="tags:webapp" />);
      const dot = getTagDots()[0] as HTMLElement;
      expect(dot.style.boxShadow).not.toBe('none');
    });
  });

  describe('click interaction', () => {
    it('calls onTagClick with the tag name when a dot is clicked', () => {
      const onTagClick = vi.fn();
      render(() => <TagBadges tags={['web', 'db']} onTagClick={onTagClick} />);
      // Click the first dot's parent (the click handler is on the wrapper div)
      const dot = getTagDots()[0] as HTMLElement;
      fireEvent.click(dot.parentElement!);
      expect(onTagClick).toHaveBeenCalledWith('web');
    });

    it('calls onTagClick for a hidden tag shown as dot (1 hidden case)', () => {
      const onTagClick = vi.fn();
      render(() => <TagBadges tags={['a', 'b', 'c', 'd']} onTagClick={onTagClick} />);
      // 4th dot is the "hidden" one shown as a dot
      const dots = getTagDots();
      fireEvent.click(dots[3].parentElement!);
      expect(onTagClick).toHaveBeenCalledWith('d');
    });

    it('does not propagate click events (stopPropagation)', () => {
      const outerClick = vi.fn();
      const onTagClick = vi.fn();
      render(() => (
        <div onClick={outerClick}>
          <TagBadges tags={['web']} onTagClick={onTagClick} />
        </div>
      ));
      const dot = getTagDots()[0] as HTMLElement;
      fireEvent.click(dot.parentElement!);
      expect(onTagClick).toHaveBeenCalledWith('web');
      expect(outerClick).not.toHaveBeenCalled();
    });

    it('does not throw when onTagClick is not provided', () => {
      render(() => <TagBadges tags={['web']} />);
      const dot = getTagDots()[0] as HTMLElement;
      expect(() => fireEvent.click(dot.parentElement!)).not.toThrow();
    });
  });

  describe('tooltip behavior', () => {
    it('shows tooltip on mouseenter of a tag dot', () => {
      render(() => <TagBadges tags={['web']} />);
      const dot = getTagDots()[0] as HTMLElement;
      fireEvent.mouseEnter(dot.parentElement!);
      expect(showTooltipMock).toHaveBeenCalledTimes(1);
      expect(showTooltipMock).toHaveBeenCalledWith('web', expect.any(Number), expect.any(Number), {
        align: 'center',
        direction: 'up',
      });
    });

    it('hides tooltip on mouseleave of a tag dot', () => {
      render(() => <TagBadges tags={['web']} />);
      const dot = getTagDots()[0] as HTMLElement;
      fireEvent.mouseLeave(dot.parentElement!);
      expect(hideTooltipMock).toHaveBeenCalledTimes(1);
    });

    it('shows tooltip with joined hidden tags on +X hover', () => {
      render(() => <TagBadges tags={['a', 'b', 'c', 'd', 'e']} />);
      const overflowIndicator = screen.getByText('+2');
      fireEvent.mouseEnter(overflowIndicator.parentElement!);
      expect(showTooltipMock).toHaveBeenCalledWith('d\ne', expect.any(Number), expect.any(Number), {
        align: 'center',
        direction: 'up',
        maxWidth: 260,
      });
    });

    it('hides tooltip on mouseleave of +X indicator', () => {
      render(() => <TagBadges tags={['a', 'b', 'c', 'd', 'e']} />);
      const overflowIndicator = screen.getByText('+2');
      fireEvent.mouseLeave(overflowIndicator.parentElement!);
      expect(hideTooltipMock).toHaveBeenCalledTimes(1);
    });
  });

  describe('edge cases', () => {
    it('handles a single tag', () => {
      render(() => <TagBadges tags={['only']} />);
      const dots = getTagDots();
      expect(dots.length).toBe(1);
    });

    it('handles many tags gracefully', () => {
      const tags = Array.from({ length: 50 }, (_, i) => `tag-${i}`);
      render(() => <TagBadges tags={tags} />);
      const dots = getTagDots();
      // Default maxVisible=3 → 3 dots shown
      expect(dots.length).toBe(3);
      expect(screen.getByText('+47')).toBeInTheDocument();
    });
  });
});
