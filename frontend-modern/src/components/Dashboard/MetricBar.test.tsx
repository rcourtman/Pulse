import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@solidjs/testing-library';
import { MetricBar } from './MetricBar';

// Mock Stores
const mockUseMetricsViewMode = vi.fn();
vi.mock('@/stores/metricsViewMode', () => ({
    useMetricsViewMode: () => mockUseMetricsViewMode()
}));

const mockGetMetricHistory = vi.fn();
vi.mock('@/stores/metricsHistory', () => ({
    getMetricHistoryForRange: (...args: any[]) => mockGetMetricHistory(...args),
    getMetricsVersion: vi.fn()
}));

// Mock Sparkline
vi.mock('@/components/shared/Sparkline', () => ({
    Sparkline: (props: any) => <div data-testid="sparkline" title={`Data count: ${props.data?.length ?? 0}`}>{props.metric}</div>
}));

// Mock ResizeObserver
let resizeCallback: ResizeObserverCallback | undefined;
const mockObserve = vi.fn();
const mockDisconnect = vi.fn();

global.ResizeObserver = class ResizeObserver {
    constructor(cb: ResizeObserverCallback) {
        resizeCallback = cb;
    }
    observe = mockObserve;
    disconnect = mockDisconnect;
    unobserve = vi.fn();
};

describe('MetricBar', () => {
    beforeEach(() => {
        mockUseMetricsViewMode.mockReturnValue({
            viewMode: () => 'bars',
            timeRange: () => '1h'
        });
        mockGetMetricHistory.mockReturnValue([]);

        // Default Mock offsetWidth
        Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 100 });

        resizeCallback = undefined;
        vi.clearAllMocks();
    });

    afterEach(() => {
        cleanup();
    });

    it('renders basic bar with label', () => {
        render(() => <MetricBar value={50} label="50%" type="cpu" />);
        expect(screen.getByText('50%')).toBeInTheDocument();
        const textEl = screen.getByText('50%');
        const container = textEl.closest('.relative') as HTMLElement;
        const bar = container.firstElementChild as HTMLElement;
        expect(bar).toHaveStyle({ width: '50%' });
    });

    it('renders correct color classes for CPU', () => {
        let result = render(() => <MetricBar value={50} label="val" type="cpu" />);
        let bar = result.container.querySelector('.bg-green-500\\/60');
        expect(bar).toBeInTheDocument();
        result.unmount();

        result = render(() => <MetricBar value={80} label="val" type="cpu" />);
        bar = result.container.querySelector('.bg-yellow-500\\/60');
        expect(bar).toBeInTheDocument();
        result.unmount();

        result = render(() => <MetricBar value={90} label="val" type="cpu" />);
        bar = result.container.querySelector('.bg-red-500\\/60');
        expect(bar).toBeInTheDocument();
        result.unmount();
    });

    it('renders correct color classes for Memory', () => {
        let result = render(() => <MetricBar value={50} label="val" type="memory" />);
        let bar = result.container.querySelector('.bg-green-500\\/60');
        expect(bar).toBeInTheDocument();
        result.unmount();

        result = render(() => <MetricBar value={75} label="val" type="memory" />);
        bar = result.container.querySelector('.bg-yellow-500\\/60');
        expect(bar).toBeInTheDocument();
        result.unmount();

        result = render(() => <MetricBar value={85} label="val" type="memory" />);
        bar = result.container.querySelector('.bg-red-500\\/60');
        expect(bar).toBeInTheDocument();
        result.unmount();
    });

    it('renders correct color classes for Disk', () => {
        let result = render(() => <MetricBar value={50} label="val" type="disk" />);
        let bar = result.container.querySelector('.bg-green-500\\/60');
        expect(bar).toBeInTheDocument();
        result.unmount();

        result = render(() => <MetricBar value={80} label="val" type="disk" />);
        bar = result.container.querySelector('.bg-yellow-500\\/60');
        expect(bar).toBeInTheDocument();
        result.unmount();

        result = render(() => <MetricBar value={90} label="val" type="disk" />);
        bar = result.container.querySelector('.bg-red-500\\/60');
        expect(bar).toBeInTheDocument();
        result.unmount();
    });

    it('renders correct color classes for Generic/Default', () => {
        let result = render(() => <MetricBar value={50} label="val" />);
        let bar = result.container.querySelector('.bg-green-500\\/60');
        expect(bar).toBeInTheDocument();
        result.unmount();

        result = render(() => <MetricBar value={75} label="val" />);
        bar = result.container.querySelector('.bg-yellow-500\\/60');
        expect(bar).toBeInTheDocument();
        result.unmount();

        result = render(() => <MetricBar value={90} label="val" />);
        bar = result.container.querySelector('.bg-red-500\\/60');
        expect(bar).toBeInTheDocument();
        result.unmount();
    });

    it('toggles sparkline view mode', () => {
        mockUseMetricsViewMode.mockReturnValue({
            viewMode: () => 'sparklines',
            timeRange: () => '1h'
        });

        render(() => <MetricBar value={50} label="val" resourceId="node1" type="cpu" />);
        expect(screen.getByTestId('sparkline')).toBeInTheDocument();
        expect(screen.queryByText('val')).not.toBeInTheDocument();
    });

    it('falls back to bars if resourceId missing in sparkline mode', () => {
        mockUseMetricsViewMode.mockReturnValue({
            viewMode: () => 'sparklines',
            timeRange: () => '1h'
        });
        render(() => <MetricBar value={50} label="val" />); // No resourceId
        expect(screen.queryByTestId('sparkline')).not.toBeInTheDocument();
        expect(screen.getByText('val')).toBeInTheDocument();
    });

    it('shows sublabel when space permits', () => {
        render(() => <MetricBar value={50} label="val" sublabel="sub" />);
        const sub = screen.getByText('(sub)');
        expect(sub).toBeInTheDocument();
    });

    it('hides sublabel when space constrained', () => {
        // Mock small width
        Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 20 });
        // Must use long text to ensure it definitely doesn't fit in 20px
        render(() => <MetricBar value={50} label="VeryLongLabel" sublabel="LongSublabel" />);
        expect(screen.queryByText('(LongSublabel)')).not.toBeInTheDocument();
    });

    it('updates sublabel visibility on resize', async () => {
        // Start small -> Hidden
        Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 20 });

        render(() => <MetricBar value={50} label="Label" sublabel="Sub" />);
        expect(screen.queryByText('(Sub)')).not.toBeInTheDocument();

        // Resize to 200px
        resizeCallback?.([{ contentRect: { width: 200 } } as ResizeObserverEntry], {} as ResizeObserver);

        expect(await screen.findByText('(Sub)')).toBeInTheDocument();
    });

    it('passes metric history to Sparkline', () => {
        const dummyData = [{ value: 1, timestamp: 100 }];
        mockUseMetricsViewMode.mockReturnValue({
            viewMode: () => 'sparklines',
            timeRange: () => '1h'
        });
        mockGetMetricHistory.mockReturnValue(dummyData);

        render(() => <MetricBar value={50} label="val" resourceId="r1" type="cpu" />);
        const spark = screen.getByTestId('sparkline');
        expect(spark).toHaveAttribute('title', 'Data count: 1');
    });

    it('handles sparkline metric types correctly', () => {
        mockUseMetricsViewMode.mockReturnValue({
            viewMode: () => 'sparklines',
            timeRange: () => '1h'
        });

        // Case: Generic -> cpu logic
        render(() => <MetricBar value={50} label="val" resourceId="r1" type="generic" />);
        expect(screen.getByText('cpu')).toBeInTheDocument();
        cleanup();

        // Case: Undefined -> cpu logic
        render(() => <MetricBar value={50} label="val" resourceId="r2" />);
        expect(screen.getByText('cpu')).toBeInTheDocument();
        cleanup();

        // Case: memory -> memory
        render(() => <MetricBar value={50} label="val" resourceId="r3" type="memory" />);
        expect(screen.getByText('memory')).toBeInTheDocument();
    });
});
