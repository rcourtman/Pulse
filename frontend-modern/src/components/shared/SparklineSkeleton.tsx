import { Component } from 'solid-js';

interface SparklineSkeletonProps {
    class?: string;
}

export const SparklineSkeleton: Component<SparklineSkeletonProps> = (props) => {
    return (
        <div
            class={`w-full h-full min-h-[88px] flex flex-col ${props.class || ''}`}
            data-testid="sparkline-skeleton"
        >
            <div class="relative flex-1 min-h-0 pl-7 pr-3 max-w-full overflow-hidden">
                {/* Y-axis placeholders */}
                <div class="absolute inset-y-0 left-0 w-7 flex flex-col justify-between py-[2%]">
                    <div class="h-1.5 w-4 bg-surface-hover rounded animate-pulse" />
                    <div class="h-1.5 w-4 bg-surface-hover rounded animate-pulse" />
                    <div class="h-1.5 w-4 bg-surface-hover rounded animate-pulse" />
                    <div class="h-1.5 w-4 bg-surface-hover rounded animate-pulse" />
                </div>

                {/* Center line / chart placeholder */}
                <div class="h-full w-full relative flex items-center justify-center border-l border-b border-transparent">
                    {/* Subtle grid lines */}
                    <div class="absolute w-full top-[25%] border-t border-border-subtle" />
                    <div class="absolute w-full top-[50%] border-t border-border" />
                    <div class="absolute w-full top-[75%] border-t border-border-subtle" />

                    {/* Animated line representing the graph */}
                    <svg
                        class="w-full h-full animate-pulse text-slate-200 dark:text-slate-700"
                        preserveAspectRatio="none"
                        viewBox="0 0 200 100"
                        fill="none"
                    >
                        <path
                            d="M0,50 Q25,30 50,60 T100,40 T150,70 T200,50"
                            stroke="currentColor"
                            stroke-width="2"
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            vector-effect="non-scaling-stroke"
                        />
                        {/* Soft area under the skeleton */}
                        <path
                            d="M0,50 Q25,30 50,60 T100,40 T150,70 T200,50 L200,100 L0,100 Z"
                            fill="currentColor"
                            fill-opacity="0.1"
                        />
                    </svg>
                </div>
            </div>

            {/* X-axis placeholders */}
            <div class="h-4 pl-7 pr-3 mt-1 flex justify-between">
                <div class="h-1.5 w-6 bg-surface-hover rounded animate-pulse" />
                <div class="h-1.5 w-6 bg-surface-hover rounded animate-pulse" />
                <div class="h-1.5 w-4 bg-surface-hover rounded animate-pulse" />
            </div>
        </div>
    );
};
