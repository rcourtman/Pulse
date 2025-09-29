import { Component, createSignal, onCleanup, onMount, Show } from 'solid-js';

interface NotificationToastProps {
  message: string;
  type: 'success' | 'error' | 'info';
  duration?: number;
  onClose?: () => void;
}

const NotificationToast: Component<NotificationToastProps> = (props) => {
  const [isVisible, setIsVisible] = createSignal(true);
  const [isLeaving, setIsLeaving] = createSignal(false);

  const handleClose = () => {
    setIsLeaving(true);
    setTimeout(() => {
      setIsVisible(false);
      props.onClose?.();
    }, 300);
  };

  onMount(() => {
    if (props.duration && props.duration > 0) {
      const timer = setTimeout(() => {
        handleClose();
      }, props.duration);

      onCleanup(() => clearTimeout(timer));
    }
  });

  const iconColor = () => {
    switch (props.type) {
      case 'success':
        return 'text-green-400';
      case 'error':
        return 'text-red-400';
      default:
        return 'text-blue-400';
    }
  };

  const icon = () => {
    switch (props.type) {
      case 'success':
        return (
          <div class="relative">
            <div class="absolute inset-0 bg-green-400 rounded-full blur-xl opacity-50 animate-pulse"></div>
            <svg class="w-6 h-6 relative" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2.5"
                d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
          </div>
        );
      case 'error':
        return (
          <div class="relative">
            <div class="absolute inset-0 bg-red-400 rounded-full blur-xl opacity-50 animate-pulse"></div>
            <svg class="w-6 h-6 relative" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2.5"
                d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
          </div>
        );
      default:
        return (
          <div class="relative">
            <div class="absolute inset-0 bg-blue-400 rounded-full blur-xl opacity-50 animate-pulse"></div>
            <svg class="w-6 h-6 relative" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2.5"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
          </div>
        );
    }
  };

  return (
    <Show when={isVisible()}>
      <div
        class={`
          fixed top-4 right-4 z-50 
          backdrop-blur-xl bg-white/10 dark:bg-gray-900/30
          border border-white/20 dark:border-gray-700/30
          px-5 py-4 rounded-2xl shadow-2xl 
          flex items-center gap-4 
          min-w-[320px] max-w-[500px]
          transition-all duration-500 ease-out
          ${isLeaving() ? 'opacity-0 translate-x-full scale-95' : 'opacity-100 translate-x-0 scale-100'}
          animate-slide-in-glass
        `}
        style={{
          background:
            'linear-gradient(135deg, rgba(255,255,255,0.1) 0%, rgba(255,255,255,0.05) 100%)',
          'box-shadow':
            '0 8px 32px 0 rgba(31, 38, 135, 0.37), inset 0 0 0 1px rgba(255,255,255,0.1)',
        }}
      >
        <div class={`${iconColor()} flex-shrink-0`}>{icon()}</div>
        <div class="flex-1">
          <p class="text-gray-800 dark:text-gray-100 font-medium text-sm leading-relaxed">
            {props.message}
          </p>
        </div>
        <button
          type="button"
          onClick={handleClose}
          class="flex-shrink-0 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-white/10 rounded-lg p-1.5 transition-all duration-200"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      </div>
    </Show>
  );
};

export default NotificationToast;
