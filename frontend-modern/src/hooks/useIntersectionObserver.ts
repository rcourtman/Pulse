import { onMount, onCleanup, createSignal } from 'solid-js';

interface UseIntersectionObserverOptions {
  threshold?: number;
  rootMargin?: string;
  root?: Element | null;
}

export function useIntersectionObserver(
  ref: () => HTMLElement | undefined,
  options: UseIntersectionObserverOptions = {},
) {
  const [isIntersecting, setIsIntersecting] = createSignal(false);
  let observer: IntersectionObserver | undefined;

  onMount(() => {
    const element = ref();
    if (!element || typeof window === 'undefined' || !window.IntersectionObserver) {
      // If IntersectionObserver is not supported, assume element is visible
      setIsIntersecting(true);
      return;
    }

    observer = new IntersectionObserver(
      ([entry]) => {
        setIsIntersecting(entry.isIntersecting);
      },
      {
        threshold: options.threshold || 0,
        rootMargin: options.rootMargin || '50px',
        root: options.root || null,
      },
    );

    observer.observe(element);
  });

  onCleanup(() => {
    if (observer) {
      observer.disconnect();
    }
  });

  return isIntersecting;
}
