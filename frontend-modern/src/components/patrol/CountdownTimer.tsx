import { createSignal, onCleanup, onMount, createEffect } from 'solid-js';

interface CountdownTimerProps {
    targetDate: string;
    prefix?: string;
    // Prefer `class` (Solid convention). `className` is supported for legacy call-sites.
    class?: string;
    className?: string;
}

export function CountdownTimer(props: CountdownTimerProps) {
    const [timeLeft, setTimeLeft] = createSignal('');

    const calculateTimeLeft = () => {
        const now = new Date();
        const target = new Date(props.targetDate);
        const diffMs = target.getTime() - now.getTime();

        if (diffMs <= 0) {
            return 'Due now';
        }

        const diffSecs = Math.floor(diffMs / 1000);
        const hours = Math.floor(diffSecs / 3600);
        const minutes = Math.floor((diffSecs % 3600) / 60);
        const seconds = diffSecs % 60;

        if (hours > 0) {
            return `${hours}h ${minutes}m`;
        }

        return `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
    };

    let timer: number | undefined;

    const update = () => {
        setTimeLeft(calculateTimeLeft());
    };

    createEffect(() => {
        // Reset/update immediately when prop changes
        if (props.targetDate) {
            update();
        }
    });

    onMount(() => {
        update();
        timer = setInterval(update, 1000) as unknown as number;
    });

    onCleanup(() => {
        clearInterval(timer);
    });

    return (
        <span class={props.class ?? props.className}>
            {props.prefix}{timeLeft()}
        </span>
    );
}
