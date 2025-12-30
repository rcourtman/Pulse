import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@solidjs/testing-library';
import { ThresholdSlider } from './ThresholdSlider';

// Mock Utils
vi.mock('@/utils/temperature', () => ({
    formatTemperature: (val: number) => `${val}°C`,
    getTemperatureSymbol: () => '°C'
}));

describe('ThresholdSlider', () => {
    afterEach(() => {
        cleanup();
    });

    it('renders with correct value for percentage types', () => {
        const onChange = vi.fn();
        render(() => <ThresholdSlider value={50} onChange={onChange} type="cpu" />);

        // Thumb text
        expect(screen.getByText('50%')).toBeInTheDocument();
        // Input accessible via title or just implicit role
        // The input is opacity-0 but exists
        // It has a title
        const input = screen.getByTitle('CPU: 50%');
        expect(input).toBeInTheDocument();
        expect(input).toHaveValue('50');
    });

    it('renders with correct value for temperature', () => {
        const onChange = vi.fn();
        render(() => <ThresholdSlider value={45} onChange={onChange} type="temperature" />);

        expect(screen.getByText('45°C')).toBeInTheDocument();
        const input = screen.getByTitle('Temperature: 45°C');
        expect(input).toBeInTheDocument();
    });

    it('triggers onChange when input changes', () => {
        const onChange = vi.fn();
        render(() => <ThresholdSlider value={50} onChange={onChange} type="memory" />);

        const input = screen.getByTitle('MEMORY: 50%') as HTMLInputElement;
        fireEvent.input(input, { target: { value: '75' } });

        expect(onChange).toHaveBeenCalledWith(75);
    });

    it('applies correct position and styling', () => {
        // value 50 -> 50% left, translate -50%
        render(() => <ThresholdSlider value={50} onChange={vi.fn()} type="cpu" min={0} max={100} />);

        const thumb = screen.getByText('50%').closest('.absolute.pointer-events-none');
        expect(thumb).toHaveStyle({ left: '50%' });
        expect(thumb).toHaveStyle({ transform: 'translateY(-50%) translateX(-50%)' });
    });

    it('handles edge positions logic (left)', () => {
        // value 0 -> 0% left, translate 0%
        render(() => <ThresholdSlider value={0} onChange={vi.fn()} type="cpu" />);

        const thumb = screen.getByText('0%').closest('.absolute.pointer-events-none');
        expect(thumb).toHaveStyle({ left: '0%' });
        expect(thumb).toHaveStyle({ transform: 'translateY(-50%) translateX(0%)' });
    });

    it('handles edge positions logic (right)', () => {
        // value 100 -> 100% left, translate -100%
        render(() => <ThresholdSlider value={100} onChange={vi.fn()} type="cpu" />);

        const thumb = screen.getByText('100%').closest('.absolute.pointer-events-none');
        expect(thumb).toHaveStyle({ left: '100%' });
        expect(thumb).toHaveStyle({ transform: 'translateY(-50%) translateX(-100%)' });
    });

    it('handles mouse drag events for scrolling lock', () => {
        // Asserting scroll lock logic via mocks on window/document is complex.
        // We can verify event listeners are added/removed if we spy on them.
        const addSpy = vi.spyOn(document, 'addEventListener');
        const removeSpy = vi.spyOn(document, 'removeEventListener');

        render(() => <ThresholdSlider value={50} onChange={vi.fn()} type="disk" />);
        const input = screen.getByTitle('DISK: 50%');

        // Mouse Down
        fireEvent.mouseDown(input);
        expect(addSpy).toHaveBeenCalledWith('mouseup', expect.any(Function));

        // Mouse Up
        const mouseUpHandler = addSpy.mock.calls.find(c => c[0] === 'mouseup')![1] as EventListener;
        mouseUpHandler({} as Event); // Simulate up

        expect(removeSpy).toHaveBeenCalledWith('mouseup', expect.any(Function));
    });

    it('prevents default wheel event on input always', () => {
        render(() => <ThresholdSlider value={50} onChange={vi.fn()} type="disk" />);
        const input = screen.getByTitle('DISK: 50%');

        const evt = new WheelEvent('wheel', { bubbles: true, cancelable: true });
        const spy = vi.spyOn(evt, 'preventDefault');

        input.dispatchEvent(evt);
        expect(spy).toHaveBeenCalled();
    });

    it('prevents default wheel event on container when dragging', () => {
        render(() => <ThresholdSlider value={50} onChange={vi.fn()} type="disk" />);
        const input = screen.getByTitle('DISK: 50%');
        // The container happens to be the parent of the input (which is absolute inset-0)
        // Input is child of div (container).
        const container = input.parentElement!;

        // Start dragging
        fireEvent.mouseDown(input);

        const evt = new WheelEvent('wheel', { bubbles: true, cancelable: true });
        const spy = vi.spyOn(evt, 'preventDefault');

        container.dispatchEvent(evt);
        expect(spy).toHaveBeenCalled();
    });

    it('executes scroll lock handler while dragging', () => {
        const scrollToSpy = vi.spyOn(window, 'scrollTo').mockImplementation(() => { });
        render(() => <ThresholdSlider value={50} onChange={vi.fn()} type="disk" />);
        const input = screen.getByTitle('DISK: 50%');

        // Start dragging
        fireEvent.mouseDown(input);

        // Trigger scroll
        fireEvent.scroll(window);

        expect(scrollToSpy).toHaveBeenCalled();
    });

    it('applies correct color classes', () => {
        const { unmount } = render(() => <ThresholdSlider value={50} onChange={vi.fn()} type="cpu" />);
        // Blue
        expect(screen.getByText('50%').closest('.text-blue-500')).toBeInTheDocument();
        unmount();

        render(() => <ThresholdSlider value={50} onChange={vi.fn()} type="temperature" />);
        // Rose
        expect(screen.getByText('50°C').closest('.text-rose-500')).toBeInTheDocument();
    });
});
