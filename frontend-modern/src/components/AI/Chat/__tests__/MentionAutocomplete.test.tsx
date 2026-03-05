import { describe, expect, it, vi, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { MentionAutocomplete } from '../MentionAutocomplete';
import type { MentionResource } from '../MentionAutocomplete';

afterEach(cleanup);

function makeResource(overrides?: Partial<MentionResource>): MentionResource {
  return {
    id: 'vm-100',
    name: 'web-server',
    type: 'vm',
    status: 'running',
    node: 'pve1',
    ...overrides,
  };
}

const defaultResources: MentionResource[] = [
  makeResource({ id: 'vm-100', name: 'web-server', type: 'vm', status: 'running', node: 'pve1' }),
  makeResource({
    id: 'ct-200',
    name: 'db-container',
    type: 'system-container',
    status: 'running',
    node: 'pve2',
  }),
  makeResource({ id: 'node-1', name: 'pve1', type: 'node', status: 'running' }),
  makeResource({ id: 'stor-1', name: 'local-lvm', type: 'storage', node: undefined }),
  makeResource({
    id: 'docker-1',
    name: 'nginx-proxy',
    type: 'docker',
    status: 'running',
    node: 'docker-host',
  }),
  makeResource({ id: 'host-1', name: 'bare-metal-01', type: 'agent', status: 'stopped' }),
];

const defaultPosition = { top: 100, left: 200 };

function renderAutocomplete(overrides?: Record<string, unknown>) {
  const props = {
    query: '',
    resources: defaultResources,
    position: defaultPosition,
    onSelect: vi.fn(),
    onClose: vi.fn(),
    visible: true,
    ...overrides,
  };
  return { ...render(() => <MentionAutocomplete {...props} />), props };
}

describe('MentionAutocomplete', () => {
  describe('visibility', () => {
    it('renders the dropdown when visible is true and resources exist', () => {
      renderAutocomplete();
      expect(screen.getByText('Resources')).toBeInTheDocument();
    });

    it('does not render when visible is false', () => {
      renderAutocomplete({ visible: false });
      expect(screen.queryByText('Resources')).not.toBeInTheDocument();
    });

    it('does not render when no resources match the query', () => {
      renderAutocomplete({ query: 'zzz-nonexistent-resource' });
      expect(screen.queryByText('Resources')).not.toBeInTheDocument();
    });
  });

  describe('filtering', () => {
    it('shows all resources (up to 10) when query is empty', () => {
      renderAutocomplete({ query: '' });
      for (const r of defaultResources) {
        expect(screen.getByText(r.name)).toBeInTheDocument();
      }
    });

    it('filters resources by name case-insensitively', () => {
      renderAutocomplete({ query: 'WEB' });
      expect(screen.getByText('web-server')).toBeInTheDocument();
      expect(screen.queryByText('db-container')).not.toBeInTheDocument();
    });

    it('filters by partial match', () => {
      renderAutocomplete({ query: 'container' });
      expect(screen.getByText('db-container')).toBeInTheDocument();
      expect(screen.queryByText('web-server')).not.toBeInTheDocument();
    });

    it('limits results to 10 items', () => {
      const manyResources = Array.from({ length: 15 }, (_, i) =>
        makeResource({ id: `vm-${i}`, name: `server-${i}` }),
      );
      renderAutocomplete({ resources: manyResources, query: '' });

      // First 10 should be present
      for (let i = 0; i < 10; i++) {
        expect(screen.getByText(`server-${i}`)).toBeInTheDocument();
      }
      // 11th and beyond should not
      expect(screen.queryByText('server-10')).not.toBeInTheDocument();
    });
  });

  describe('resource display', () => {
    it('shows the resource type label', () => {
      renderAutocomplete({ query: 'web' });
      // "vm" text is inside a div alongside " · " and node name, so use a substring match
      const resourceBtn = screen.getByText('web-server').closest('button')!;
      expect(resourceBtn.textContent).toContain('vm');
    });

    it('shows agent type label for agent resources', () => {
      renderAutocomplete({ query: 'bare-metal' });
      const resourceBtn = screen.getByText('bare-metal-01').closest('button')!;
      expect(resourceBtn.textContent).toContain('agent');
      expect(resourceBtn.textContent).not.toContain('host');
    });

    it('shows the node name when present', () => {
      renderAutocomplete({ query: 'web' });
      expect(screen.getByText('pve1', { exact: false })).toBeInTheDocument();
    });

    it('does not show node separator when node is absent', () => {
      renderAutocomplete({ query: 'local-lvm' });
      const resourceBtn = screen.getByText('local-lvm').closest('button')!;
      // Should contain "storage" but NOT " · "
      const typeDiv = resourceBtn.querySelector('.text-xs.text-muted')!;
      expect(typeDiv.textContent).toContain('storage');
      expect(typeDiv.textContent).not.toContain(' · ');
    });

    it('shows status indicator dot for resources with status', () => {
      renderAutocomplete({ query: 'web' });
      // The status dot is a span with bg-green-500 class (running)
      const resourceBtn = screen.getByText('web-server').closest('button')!;
      const statusDot = resourceBtn.querySelector('.bg-green-500');
      expect(statusDot).not.toBeNull();
    });

    it('shows red status dot for stopped resources', () => {
      renderAutocomplete({ query: 'bare-metal' });
      const resourceBtn = screen.getByText('bare-metal-01').closest('button')!;
      const statusDot = resourceBtn.querySelector('.bg-red-500');
      expect(statusDot).not.toBeNull();
    });
  });

  describe('click interaction', () => {
    it('calls onSelect when a resource is clicked', () => {
      const onSelect = vi.fn();
      renderAutocomplete({ onSelect });

      fireEvent.click(screen.getByText('web-server'));
      expect(onSelect).toHaveBeenCalledOnce();
      expect(onSelect).toHaveBeenCalledWith(defaultResources[0]);
    });

    it('calls onSelect with the correct resource for non-first items', () => {
      const onSelect = vi.fn();
      renderAutocomplete({ onSelect });

      fireEvent.click(screen.getByText('db-container'));
      expect(onSelect).toHaveBeenCalledWith(defaultResources[1]);
    });
  });

  describe('keyboard navigation', () => {
    it('selects the next item on ArrowDown', () => {
      const onSelect = vi.fn();
      renderAutocomplete({ onSelect });

      // ArrowDown to move to index 1
      fireEvent.keyDown(document, { key: 'ArrowDown' });
      // Enter to select
      fireEvent.keyDown(document, { key: 'Enter' });

      expect(onSelect).toHaveBeenCalledWith(defaultResources[1]);
    });

    it('selects the previous item on ArrowUp', () => {
      const onSelect = vi.fn();
      renderAutocomplete({ onSelect });

      // Move down twice, then up once → index 1
      fireEvent.keyDown(document, { key: 'ArrowDown' });
      fireEvent.keyDown(document, { key: 'ArrowDown' });
      fireEvent.keyDown(document, { key: 'ArrowUp' });
      fireEvent.keyDown(document, { key: 'Enter' });

      expect(onSelect).toHaveBeenCalledWith(defaultResources[1]);
    });

    it('does not go below 0 on ArrowUp at the top', () => {
      const onSelect = vi.fn();
      renderAutocomplete({ onSelect });

      // ArrowUp at index 0 should stay at 0
      fireEvent.keyDown(document, { key: 'ArrowUp' });
      fireEvent.keyDown(document, { key: 'Enter' });

      expect(onSelect).toHaveBeenCalledWith(defaultResources[0]);
    });

    it('does not go past last item on ArrowDown', () => {
      const onSelect = vi.fn();
      const twoResources = defaultResources.slice(0, 2);
      renderAutocomplete({ onSelect, resources: twoResources });

      // ArrowDown 5 times should cap at index 1
      for (let i = 0; i < 5; i++) {
        fireEvent.keyDown(document, { key: 'ArrowDown' });
      }
      fireEvent.keyDown(document, { key: 'Enter' });

      expect(onSelect).toHaveBeenCalledWith(twoResources[1]);
    });

    it('selects on Tab key', () => {
      const onSelect = vi.fn();
      renderAutocomplete({ onSelect });

      fireEvent.keyDown(document, { key: 'Tab' });
      expect(onSelect).toHaveBeenCalledWith(defaultResources[0]);
    });

    it('calls onClose on Escape', () => {
      const onClose = vi.fn();
      renderAutocomplete({ onClose });

      fireEvent.keyDown(document, { key: 'Escape' });
      expect(onClose).toHaveBeenCalledOnce();
    });

    it('does not respond to keyboard events when not visible', () => {
      const onSelect = vi.fn();
      const onClose = vi.fn();
      // Render as visible first to register the listener, then check behavior
      // The component guards with `if (!props.visible) return;` inside handleKeyDown
      renderAutocomplete({ visible: false, onSelect, onClose });

      // Since the component is not visible, the keydown listener is not registered,
      // so dispatching events should have no effect
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));

      expect(onSelect).not.toHaveBeenCalled();
      expect(onClose).not.toHaveBeenCalled();
    });
  });

  describe('mouse hover', () => {
    it('updates selected index on mouse enter', () => {
      const onSelect = vi.fn();
      renderAutocomplete({ onSelect });

      // Hover over the second item
      fireEvent.mouseEnter(screen.getByText('db-container').closest('button')!);
      // Then press Enter to confirm it's selected
      fireEvent.keyDown(document, { key: 'Enter' });

      expect(onSelect).toHaveBeenCalledWith(defaultResources[1]);
    });
  });

  describe('positioning', () => {
    it('applies position styles to the dropdown container', () => {
      renderAutocomplete({ position: { top: 50, left: 120 } });
      const container = screen.getByText('Resources').closest('div[style]')!;
      expect(container).toHaveStyle({ bottom: '50px', left: '120px' });
    });
  });

  describe('footer hints', () => {
    it('shows keyboard navigation hints', () => {
      renderAutocomplete();
      // Footer text is split across span + text nodes; check the footer container text
      const footer = document.querySelector('.border-t.border-border.text-xs.text-muted')!;
      expect(footer).not.toBeNull();
      expect(footer.textContent).toContain('navigate');
      expect(footer.textContent).toContain('select');
      expect(footer.textContent).toContain('close');
    });
  });

  describe('type icons', () => {
    it('renders an SVG icon for each resource type', () => {
      renderAutocomplete();
      // Scope to the scrollable resource list container only
      const listContainer = document.querySelector('.max-h-\\[240px\\].overflow-y-auto')!;
      expect(listContainer).not.toBeNull();
      const resourceButtons = listContainer.querySelectorAll('button');
      expect(resourceButtons.length).toBe(defaultResources.length);
      resourceButtons.forEach((btn) => {
        const svg = btn.querySelector('svg');
        expect(svg).not.toBeNull();
      });
    });
  });

  describe('edge cases', () => {
    it('handles empty resources list', () => {
      renderAutocomplete({ resources: [] });
      expect(screen.queryByText('Resources')).not.toBeInTheDocument();
    });

    it('handles undefined status gracefully — no status dot rendered', () => {
      const resources = [makeResource({ status: undefined })];
      renderAutocomplete({ resources });
      expect(screen.getByText('web-server')).toBeInTheDocument();
      // When status is undefined, <Show when={resource.status}> hides the dot
      const resourceBtn = screen.getByText('web-server').closest('button')!;
      const statusDot = resourceBtn.querySelector('.rounded-full');
      expect(statusDot).toBeNull();
    });

    it('handles undefined node gracefully', () => {
      const resources = [makeResource({ node: undefined })];
      renderAutocomplete({ resources });
      expect(screen.getByText('web-server')).toBeInTheDocument();
    });

    it('handles paused status', () => {
      const resources = [makeResource({ status: 'paused' })];
      renderAutocomplete({ resources });
      const resourceBtn = screen.getByText('web-server').closest('button')!;
      const statusDot = resourceBtn.querySelector('.bg-yellow-500');
      expect(statusDot).not.toBeNull();
    });

    it('handles unknown status with default color', () => {
      const resources = [makeResource({ status: 'migrating' })];
      renderAutocomplete({ resources });
      const resourceBtn = screen.getByText('web-server').closest('button')!;
      const statusDot = resourceBtn.querySelector('.bg-slate-400');
      expect(statusDot).not.toBeNull();
    });
  });
});
