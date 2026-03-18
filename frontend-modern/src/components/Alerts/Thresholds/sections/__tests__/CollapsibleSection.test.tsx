import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { CollapsibleSection, SectionActionButton, NestedGroupHeader } from '../CollapsibleSection';
import {
  ALERT_THRESHOLDS_SECTION_DISABLED_LABEL,
  ALERT_THRESHOLDS_SECTION_UNSAVED_CHANGES_TITLE,
} from '@/utils/alertThresholdsSectionPresentation';

afterEach(() => {
  cleanup();
});

// ---------------------------------------------------------------------------
// CollapsibleSection
// ---------------------------------------------------------------------------
describe('CollapsibleSection', () => {
  it('renders title and children when expanded', () => {
    render(() => (
      <CollapsibleSection id="test" title="Nodes">
        <p>Child content</p>
      </CollapsibleSection>
    ));

    expect(screen.getByText('Nodes')).toBeInTheDocument();
    expect(screen.getByText('Child content')).toBeInTheDocument();
  });

  it('uses default data-testid from id prop', () => {
    const { container } = render(() => (
      <CollapsibleSection id="vms" title="VMs">
        <span />
      </CollapsibleSection>
    ));

    expect(container.querySelector('[data-testid="section-vms"]')).toBeInTheDocument();
  });

  it('uses custom testId when provided', () => {
    const { container } = render(() => (
      <CollapsibleSection id="vms" title="VMs" testId="custom-id">
        <span />
      </CollapsibleSection>
    ));

    expect(container.querySelector('[data-testid="custom-id"]')).toBeInTheDocument();
  });

  it('shows resource count badge when resourceCount is provided', () => {
    render(() => (
      <CollapsibleSection id="vms" title="VMs" resourceCount={5}>
        <span />
      </CollapsibleSection>
    ));

    expect(screen.getByText('5')).toBeInTheDocument();
  });

  it('does not show resource count badge when resourceCount is undefined', () => {
    const { container } = render(() => (
      <CollapsibleSection id="vms" title="VMs">
        <span />
      </CollapsibleSection>
    ));

    // No badge element should exist — the rounded-full badge span is only rendered conditionally
    const badges = container.querySelectorAll('span.rounded-full');
    // Filter to count badges specifically (they contain numeric text)
    const countBadges = Array.from(badges).filter((el) => /^\d+$/.test(el.textContent || ''));
    expect(countBadges.length).toBe(0);
  });

  it('shows resource count badge for zero count', () => {
    render(() => (
      <CollapsibleSection id="vms" title="VMs" resourceCount={0}>
        <span />
      </CollapsibleSection>
    ));

    expect(screen.getByText('0')).toBeInTheDocument();
  });

  it('sets aria-expanded=true when section is expanded (default)', () => {
    render(() => (
      <CollapsibleSection id="test" title="Title">
        <span />
      </CollapsibleSection>
    ));

    const button = screen.getByRole('button');
    expect(button).toHaveAttribute('aria-expanded', 'true');
  });

  it('sets aria-expanded=false when collapsed prop is true', () => {
    render(() => (
      <CollapsibleSection id="test" title="Title" collapsed={true}>
        <span />
      </CollapsibleSection>
    ));

    const button = screen.getByRole('button');
    expect(button).toHaveAttribute('aria-expanded', 'false');
  });

  it('sets aria-controls to match content element id', () => {
    render(() => (
      <CollapsibleSection id="my-section" title="Title">
        <span />
      </CollapsibleSection>
    ));

    const button = screen.getByRole('button');
    expect(button).toHaveAttribute('aria-controls', 'section-content-my-section');
  });

  it('toggles collapsed state on click in uncontrolled mode', () => {
    const onToggle = vi.fn();
    render(() => (
      <CollapsibleSection id="test" title="Title" onToggle={onToggle}>
        <span />
      </CollapsibleSection>
    ));

    const button = screen.getByRole('button');
    // Initially expanded (collapsed=false)
    expect(button).toHaveAttribute('aria-expanded', 'true');

    fireEvent.click(button);
    expect(onToggle).toHaveBeenCalledTimes(1);
    expect(onToggle).toHaveBeenNthCalledWith(1, true); // now collapsed
    expect(button).toHaveAttribute('aria-expanded', 'false');

    fireEvent.click(button);
    expect(onToggle).toHaveBeenCalledTimes(2);
    expect(onToggle).toHaveBeenNthCalledWith(2, false); // now expanded again
    expect(button).toHaveAttribute('aria-expanded', 'true');
  });

  it('respects controlled collapsed prop over local state', () => {
    const onToggle = vi.fn();
    // Controlled: always collapsed
    render(() => (
      <CollapsibleSection id="test" title="Title" collapsed={true} onToggle={onToggle}>
        <span />
      </CollapsibleSection>
    ));

    const button = screen.getByRole('button');
    expect(button).toHaveAttribute('aria-expanded', 'false');

    // Click will call onToggle but since collapsed prop is still true, aria-expanded stays false
    fireEvent.click(button);
    expect(onToggle).toHaveBeenCalledTimes(1);
    expect(onToggle).toHaveBeenNthCalledWith(1, false); // wants to expand
    // Since the controlled prop is still true, the section stays collapsed
    expect(button).toHaveAttribute('aria-expanded', 'false');
  });

  it('shows "Disabled" badge when isGloballyDisabled is true', () => {
    render(() => (
      <CollapsibleSection id="test" title="Title" isGloballyDisabled={true}>
        <span />
      </CollapsibleSection>
    ));

    expect(screen.getByText(ALERT_THRESHOLDS_SECTION_DISABLED_LABEL)).toBeInTheDocument();
  });

  it('does not show "Disabled" badge by default', () => {
    render(() => (
      <CollapsibleSection id="test" title="Title">
        <span />
      </CollapsibleSection>
    ));

    expect(
      screen.queryByText(ALERT_THRESHOLDS_SECTION_DISABLED_LABEL),
    ).not.toBeInTheDocument();
  });

  it('shows unsaved changes indicator when hasChanges is true', () => {
    const { container } = render(() => (
      <CollapsibleSection id="test" title="Title" hasChanges={true}>
        <span />
      </CollapsibleSection>
    ));

    const dot = container.querySelector(
      `[title="${ALERT_THRESHOLDS_SECTION_UNSAVED_CHANGES_TITLE}"]`,
    );
    expect(dot).toBeInTheDocument();
  });

  it('does not show unsaved changes indicator by default', () => {
    const { container } = render(() => (
      <CollapsibleSection id="test" title="Title">
        <span />
      </CollapsibleSection>
    ));

    const dot = container.querySelector(
      `[title="${ALERT_THRESHOLDS_SECTION_UNSAVED_CHANGES_TITLE}"]`,
    );
    expect(dot).not.toBeInTheDocument();
  });

  it('shows subtitle when provided', () => {
    render(() => (
      <CollapsibleSection id="test" title="Title" subtitle="Some description">
        <span />
      </CollapsibleSection>
    ));

    expect(screen.getByText('Some description')).toBeInTheDocument();
  });

  it('shows icon when provided', () => {
    render(() => (
      <CollapsibleSection id="test" title="Title" icon={<span data-testid="custom-icon">IC</span>}>
        <span />
      </CollapsibleSection>
    ));

    expect(screen.getByTestId('custom-icon')).toBeInTheDocument();
  });

  it('shows empty message when resourceCount is 0 and expanded', () => {
    render(() => (
      <CollapsibleSection
        id="test"
        title="Title"
        resourceCount={0}
        emptyMessage="No resources found"
      >
        <span>Children</span>
      </CollapsibleSection>
    ));

    expect(screen.getByText('No resources found')).toBeInTheDocument();
  });

  it('does not show empty message when resourceCount is 0 but collapsed', () => {
    render(() => (
      <CollapsibleSection
        id="test"
        title="Title"
        resourceCount={0}
        collapsed={true}
        emptyMessage="No resources found"
      >
        <span>Children</span>
      </CollapsibleSection>
    ));

    // The empty message should not appear since the section is collapsed
    // (content is hidden via max-h-0 opacity-0, but the DOM element may exist)
    // The showEmpty() check requires isCollapsed() === false
    expect(screen.queryByText('No resources found')).not.toBeInTheDocument();
  });

  it('shows children when resourceCount > 0', () => {
    render(() => (
      <CollapsibleSection id="test" title="Title" resourceCount={3} emptyMessage="No resources">
        <span>Actual content</span>
      </CollapsibleSection>
    ));

    expect(screen.getByText('Actual content')).toBeInTheDocument();
    expect(screen.queryByText('No resources')).not.toBeInTheDocument();
  });

  it('shows children when no emptyMessage is provided even if resourceCount is 0', () => {
    render(() => (
      <CollapsibleSection id="test" title="Title" resourceCount={0}>
        <span>Fallback content</span>
      </CollapsibleSection>
    ));

    expect(screen.getByText('Fallback content')).toBeInTheDocument();
  });

  it('renders headerActions and clicking them does not toggle the section', () => {
    const onToggle = vi.fn();
    const actionClick = vi.fn();
    render(() => (
      <CollapsibleSection
        id="test"
        title="Title"
        onToggle={onToggle}
        headerActions={<button onClick={actionClick}>Edit</button>}
      >
        <span />
      </CollapsibleSection>
    ));

    const editButton = screen.getByText('Edit');
    fireEvent.click(editButton);

    expect(actionClick).toHaveBeenCalledTimes(1);
    // onToggle should NOT fire because the actions container uses stopPropagation
    expect(onToggle).not.toHaveBeenCalled();
  });

  it('applies globally disabled styling', () => {
    const { container } = render(() => (
      <CollapsibleSection id="test" title="Title" isGloballyDisabled={true}>
        <span />
      </CollapsibleSection>
    ));

    const wrapper = container.querySelector('[data-testid="section-test"]')!;
    expect(wrapper.className).toContain('opacity-60');
  });

  it('applies hasChanges ring styling', () => {
    const { container } = render(() => (
      <CollapsibleSection id="test" title="Title" hasChanges={true}>
        <span />
      </CollapsibleSection>
    ));

    const wrapper = container.querySelector('[data-testid="section-test"]')!;
    expect(wrapper.className).toContain('ring-2');
    expect(wrapper.className).toContain('ring-blue-400');
  });
});

// ---------------------------------------------------------------------------
// SectionActionButton
// ---------------------------------------------------------------------------
describe('SectionActionButton', () => {
  it('renders label text', () => {
    render(() => <SectionActionButton label="Edit Defaults" onClick={() => {}} />);
    expect(screen.getByText('Edit Defaults')).toBeInTheDocument();
  });

  it('calls onClick and stops propagation', () => {
    const onClick = vi.fn();
    const parentClick = vi.fn();

    render(() => (
      <div onClick={parentClick}>
        <SectionActionButton label="Save" onClick={onClick} />
      </div>
    ));

    fireEvent.click(screen.getByText('Save'));
    expect(onClick).toHaveBeenCalledTimes(1);
    expect(parentClick).not.toHaveBeenCalled();
  });

  it('renders with default variant classes', () => {
    render(() => <SectionActionButton label="Action" onClick={() => {}} />);
    const button = screen.getByRole('button');
    expect(button.className).toContain('text-slate-600');
  });

  it('renders with primary variant classes', () => {
    render(() => <SectionActionButton label="Action" onClick={() => {}} variant="primary" />);
    const button = screen.getByRole('button');
    expect(button.className).toContain('text-blue-600');
  });

  it('renders with danger variant classes', () => {
    render(() => <SectionActionButton label="Delete" onClick={() => {}} variant="danger" />);
    const button = screen.getByRole('button');
    expect(button.className).toContain('text-red-600');
  });

  it('applies disabled state', () => {
    render(() => <SectionActionButton label="Action" onClick={() => {}} disabled={true} />);
    const button = screen.getByRole('button');
    expect(button).toBeDisabled();
    expect(button.className).toContain('opacity-50');
    expect(button.className).toContain('cursor-not-allowed');
  });

  it('renders icon when provided', () => {
    render(() => (
      <SectionActionButton
        label="Edit"
        onClick={() => {}}
        icon={<span data-testid="btn-icon">I</span>}
      />
    ));
    expect(screen.getByTestId('btn-icon')).toBeInTheDocument();
  });

  it('applies title attribute when provided', () => {
    render(() => (
      <SectionActionButton label="Edit" onClick={() => {}} title="Edit defaults for this group" />
    ));
    expect(screen.getByRole('button')).toHaveAttribute('title', 'Edit defaults for this group');
  });
});

// ---------------------------------------------------------------------------
// NestedGroupHeader
// ---------------------------------------------------------------------------
describe('NestedGroupHeader', () => {
  it('renders title text', () => {
    render(() => <NestedGroupHeader title="pve-node-1" />);
    expect(screen.getByText('pve-node-1')).toBeInTheDocument();
  });

  it('shows subtitle when provided', () => {
    render(() => <NestedGroupHeader title="Node" subtitle="192.168.0.1" />);
    expect(screen.getByText('192.168.0.1')).toBeInTheDocument();
  });

  it('shows count when provided', () => {
    render(() => <NestedGroupHeader title="Node" count={12} />);
    expect(screen.getByText('(12)')).toBeInTheDocument();
  });

  it('does not show count when undefined', () => {
    render(() => <NestedGroupHeader title="Node" />);
    expect(screen.queryByText(/\(\d+\)/)).not.toBeInTheDocument();
  });

  it('shows online status indicator', () => {
    const { container } = render(() => <NestedGroupHeader title="Node" status="online" />);
    const dot = container.querySelector('[title="Online"]');
    expect(dot).toBeInTheDocument();
    expect(dot!.className).toContain('bg-emerald-500');
  });

  it('shows offline status indicator', () => {
    const { container } = render(() => <NestedGroupHeader title="Node" status="offline" />);
    const dot = container.querySelector('[title="Offline"]');
    expect(dot).toBeInTheDocument();
    expect(dot!.className).toContain('bg-red-500');
  });

  it('shows unknown status indicator', () => {
    const { container } = render(() => <NestedGroupHeader title="Node" status="unknown" />);
    const dot = container.querySelector('[title="Unknown"]');
    expect(dot).toBeInTheDocument();
    expect(dot!.className).toContain('bg-amber-500');
  });

  it('calls onToggle when clicked', () => {
    const onToggle = vi.fn();
    const { container } = render(() => <NestedGroupHeader title="Node" onToggle={onToggle} />);

    // The outermost div is the clickable header
    const header = container.firstElementChild!;
    fireEvent.click(header);
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it('does not render chevron when onToggle is not provided', () => {
    const { container } = render(() => <NestedGroupHeader title="Node" />);
    // Chevron icons have w-4 h-4 class
    const svgs = container.querySelectorAll('svg');
    expect(svgs.length).toBe(0);
  });

  it('renders different chevron icons for expanded vs collapsed', () => {
    // Render expanded (collapsed=false) — should get ChevronDown
    const expandedResult = render(() => (
      <NestedGroupHeader title="Expanded" onToggle={() => {}} collapsed={false} />
    ));
    const expandedSvg = expandedResult.container.querySelector('svg')!;
    expect(expandedSvg).toBeInTheDocument();
    const expandedMarkup = expandedSvg.outerHTML;

    cleanup();

    // Render collapsed (collapsed=true) — should get ChevronRight
    const collapsedResult = render(() => (
      <NestedGroupHeader title="Collapsed" onToggle={() => {}} collapsed={true} />
    ));
    const collapsedSvg = collapsedResult.container.querySelector('svg')!;
    expect(collapsedSvg).toBeInTheDocument();
    const collapsedMarkup = collapsedSvg.outerHTML;

    // The two icons must differ (ChevronDown vs ChevronRight have different paths)
    expect(expandedMarkup).not.toBe(collapsedMarkup);
  });

  it('renders title as a link when href is provided', () => {
    render(() => <NestedGroupHeader title="Node Link" href="https://example.com" />);

    const link = screen.getByText('Node Link').closest('a');
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute('href', 'https://example.com');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('renders title as plain text when href is not provided', () => {
    render(() => <NestedGroupHeader title="Plain Node" />);

    const text = screen.getByText('Plain Node');
    expect(text.tagName).toBe('SPAN');
  });

  it('link click does not propagate to parent toggle', () => {
    const onToggle = vi.fn();
    render(() => (
      <NestedGroupHeader title="Linked" href="https://example.com" onToggle={onToggle} />
    ));

    const link = screen.getByText('Linked');
    fireEvent.click(link);
    expect(onToggle).not.toHaveBeenCalled();
  });

  it('renders actions and action clicks do not propagate', () => {
    const onToggle = vi.fn();
    const actionClick = vi.fn();
    render(() => (
      <NestedGroupHeader
        title="Node"
        onToggle={onToggle}
        actions={<button onClick={actionClick}>Config</button>}
      />
    ));

    fireEvent.click(screen.getByText('Config'));
    expect(actionClick).toHaveBeenCalledTimes(1);
    expect(onToggle).not.toHaveBeenCalled();
  });

  it('applies hover class when onToggle is provided', () => {
    const { container } = render(() => <NestedGroupHeader title="Node" onToggle={() => {}} />);

    const wrapper = container.firstElementChild!;
    expect(wrapper.className).toContain('cursor-pointer');
    expect(wrapper.className).toContain('hover:bg-surface-hover');
  });

  it('does not apply hover class when onToggle is not provided', () => {
    const { container } = render(() => <NestedGroupHeader title="Node" />);

    const wrapper = container.firstElementChild!;
    expect(wrapper.className).not.toContain('cursor-pointer');
  });

  it('clicking header without onToggle does not throw', () => {
    const { container } = render(() => <NestedGroupHeader title="Node" />);

    const header = container.firstElementChild!;
    // Should not throw — onClick is bound to props.onToggle which is undefined
    expect(() => fireEvent.click(header)).not.toThrow();
  });
});

// ---------------------------------------------------------------------------
// SectionActionButton — disabled click behavior
// ---------------------------------------------------------------------------
describe('SectionActionButton disabled click', () => {
  it('does not invoke onClick when disabled', () => {
    const onClick = vi.fn();
    render(() => <SectionActionButton label="Action" onClick={onClick} disabled={true} />);

    const button = screen.getByRole('button');
    fireEvent.click(button);
    // Native disabled attribute prevents click handler from firing
    expect(onClick).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// CollapsibleSection — aria-controls target existence
// ---------------------------------------------------------------------------
describe('CollapsibleSection aria-controls', () => {
  it('aria-controls value matches an existing content element id', () => {
    const { container } = render(() => (
      <CollapsibleSection id="verify" title="Verify">
        <span>Content</span>
      </CollapsibleSection>
    ));

    const button = screen.getByRole('button');
    const controlsId = button.getAttribute('aria-controls')!;
    const contentEl = container.querySelector(`#${controlsId}`);
    expect(contentEl).toBeInTheDocument();
  });
});
