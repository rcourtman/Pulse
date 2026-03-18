import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { PulsePatrolLogo } from '@/components/Brand/PulsePatrolLogo';

describe('PulsePatrolLogo', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders an SVG with the default "Pulse Patrol" title', () => {
    render(() => <PulsePatrolLogo />);

    const svg = screen.getByLabelText('Pulse Patrol');
    expect(svg).toBeInTheDocument();
    expect(svg.tagName).toBe('svg');

    // The <title> element should contain the default text
    const titleEl = svg.querySelector('title');
    expect(titleEl).not.toBeNull();
    expect(titleEl!.textContent).toBe('Pulse Patrol');
  });

  it('renders a custom title when the title prop is provided', () => {
    render(() => <PulsePatrolLogo title="Custom Logo Title" />);

    const svg = screen.getByLabelText('Custom Logo Title');
    expect(svg).toBeInTheDocument();

    const titleEl = svg.querySelector('title');
    expect(titleEl!.textContent).toBe('Custom Logo Title');
  });

  it('applies the class prop to the SVG element', () => {
    render(() => <PulsePatrolLogo class="w-6 h-6 text-base-content" />);

    const svg = screen.getByLabelText('Pulse Patrol');
    expect(svg).toHaveClass('w-6', 'h-6', 'text-base-content');
  });

  it('renders without a class when none is provided', () => {
    render(() => <PulsePatrolLogo />);

    const svg = screen.getByLabelText('Pulse Patrol');
    const classAttr = svg.getAttribute('class');
    expect(classAttr ?? '').toBe('');
  });

  it('contains the infinity loop path element', () => {
    render(() => <PulsePatrolLogo />);

    const svg = screen.getByLabelText('Pulse Patrol');
    const paths = svg.querySelectorAll('path');
    expect(paths.length).toBe(1);

    // The path should have the infinity loop d attribute (two mirrored arcs through center)
    const d = paths[0].getAttribute('d');
    expect(d).toBeTruthy();
    expect(d).toContain('a4 4 0 1 0 0 8');
    expect(d).toContain('a4 4 0 1 0 0-8');
  });

  it('sets correct SVG attributes for stroke-based rendering', () => {
    render(() => <PulsePatrolLogo />);

    const svg = screen.getByLabelText('Pulse Patrol');
    expect(svg.getAttribute('viewBox')).toBe('0 0 24 24');
    expect(svg.getAttribute('fill')).toBe('none');
    expect(svg.getAttribute('stroke')).toBe('currentColor');
    expect(svg.getAttribute('stroke-width')).toBe('2');
    expect(svg.getAttribute('stroke-linecap')).toBe('round');
    expect(svg.getAttribute('stroke-linejoin')).toBe('round');
  });

  it('uses an empty string title when title is explicitly empty', () => {
    const { container } = render(() => <PulsePatrolLogo title="" />);

    // With empty string title, aria-label should be empty
    const svg = container.querySelector('svg');
    expect(svg).not.toBeNull();
    expect(svg!.getAttribute('aria-label')).toBe('');

    const titleEl = svg!.querySelector('title');
    expect(titleEl!.textContent).toBe('');
  });
});
