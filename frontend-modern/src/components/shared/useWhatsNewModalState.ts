import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { createLocalStorageBooleanSignal, STORAGE_KEYS } from '@/utils/localStorage';
import {
  presentationPolicyIsDemoMode,
  sessionPresentationPolicyResolved,
} from '@/stores/sessionPresentationPolicy';
import { WHATS_NEW_FEATURE_CARDS, WHATS_NEW_REOPEN_EVENT } from './whatsNewModalModel';

type SpotlightRect = {
  top: number;
  left: number;
  width: number;
  height: number;
};

const DESKTOP_TAB_SELECTOR_BY_TARGET = {
  dashboard: '[role="tab"][title="Environment overview and command center"]',
  infrastructure: '[role="tab"][title="All agents and nodes across platforms"]',
  workloads: '[role="tab"][title="VMs, containers, and Kubernetes workloads"]',
  storage: '[role="tab"][title="Storage pools, disks, and datastores"]',
  recovery: '[role="tab"][title="Backup, snapshot, and replication activity"]',
} as const;

const MOBILE_TAB_SELECTOR_BY_TARGET = {
  dashboard: 'button[data-tab-id="dashboard"]',
  infrastructure: 'button[data-tab-id="infrastructure"]',
  workloads: 'button[data-tab-id="workloads"]',
  storage: 'button[data-tab-id="storage"]',
  recovery: 'button[data-tab-id="recovery"]',
} as const;

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max);

const isVisibleElement = (element: Element | null): element is HTMLElement => {
  if (!(element instanceof HTMLElement)) return false;
  const rect = element.getBoundingClientRect();
  if (rect.width <= 0 || rect.height <= 0) return false;
  const style = window.getComputedStyle(element);
  return style.display !== 'none' && style.visibility !== 'hidden';
};

const selectFirstVisible = (...selectors: string[]): HTMLElement | null => {
  for (const selector of selectors) {
    const visible = Array.from(document.querySelectorAll(selector)).find((candidate) =>
      isVisibleElement(candidate),
    );
    if (visible && visible instanceof HTMLElement) {
      return visible;
    }
  }
  return null;
};

export function useWhatsNewModalState() {
  const [hasSeen, setHasSeen] = createLocalStorageBooleanSignal(
    STORAGE_KEYS.WHATS_NEW_NAV_V2_SHOWN,
    false,
  );
  const [dontShowAgain, setDontShowAgain] = createSignal(true);
  const [dismissedForSession, setDismissedForSession] = createSignal(false);
  const [stepIndex, setStepIndex] = createSignal(0);
  const [panelRef, setPanelRef] = createSignal<HTMLDivElement | null>(null);
  const [spotlightRect, setSpotlightRect] = createSignal<SpotlightRect | null>(null);

  const isOpen = () =>
    sessionPresentationPolicyResolved() &&
    !presentationPolicyIsDemoMode() &&
    !hasSeen() &&
    !dismissedForSession();

  const currentStep = createMemo(() => {
    const index = clamp(stepIndex(), 0, WHATS_NEW_FEATURE_CARDS.length - 1);
    return WHATS_NEW_FEATURE_CARDS[index];
  });

  const isFirstStep = createMemo(() => stepIndex() === 0);
  const isLastStep = createMemo(() => stepIndex() >= WHATS_NEW_FEATURE_CARDS.length - 1);

  const resetTourState = () => {
    setStepIndex(0);
    setSpotlightRect(null);
  };

  const closeTour = () => {
    resetTourState();
    if (dontShowAgain()) {
      setHasSeen(true);
      return;
    }

    setDismissedForSession(true);
  };

  const handleClose = () => {
    closeTour();
  };

  const handleNext = () => {
    if (isLastStep()) {
      closeTour();
      return;
    }
    setStepIndex((current) => clamp(current + 1, 0, WHATS_NEW_FEATURE_CARDS.length - 1));
  };

  const handlePrevious = () => {
    setStepIndex((current) => clamp(current - 1, 0, WHATS_NEW_FEATURE_CARDS.length - 1));
  };

  const handleSelectStep = (index: number) => {
    setStepIndex(clamp(index, 0, WHATS_NEW_FEATURE_CARDS.length - 1));
  };

  if (typeof window !== 'undefined') {
    const handleReopen = () => {
      setHasSeen(false);
      setDismissedForSession(false);
      setStepIndex(0);
    };
    window.addEventListener(WHATS_NEW_REOPEN_EVENT, handleReopen);
    onCleanup(() => window.removeEventListener(WHATS_NEW_REOPEN_EVENT, handleReopen));
  }

  createEffect(() => {
    if (!isOpen()) return;

    const updateSpotlight = () => {
      const step = currentStep();
      const target = selectFirstVisible(
        DESKTOP_TAB_SELECTOR_BY_TARGET[step.target],
        MOBILE_TAB_SELECTOR_BY_TARGET[step.target],
      );
      if (!target) {
        setSpotlightRect(null);
        return;
      }

      const rect = target.getBoundingClientRect();
      const padding = 10;
      setSpotlightRect({
        top: Math.max(12, rect.top - padding),
        left: Math.max(12, rect.left - padding),
        width: rect.width + padding * 2,
        height: rect.height + padding * 2,
      });
    };

    updateSpotlight();

    const resizeObserver =
      typeof ResizeObserver === 'undefined'
        ? null
        : new ResizeObserver(() => {
            updateSpotlight();
          });
    const panel = panelRef();
    if (resizeObserver && panel) {
      resizeObserver.observe(panel);
    }

    window.addEventListener('resize', updateSpotlight);
    window.addEventListener('scroll', updateSpotlight, true);

    onCleanup(() => {
      resizeObserver?.disconnect();
      window.removeEventListener('resize', updateSpotlight);
      window.removeEventListener('scroll', updateSpotlight, true);
    });
  });

  const panelStyle = createMemo(() => {
    if (typeof window === 'undefined') {
      return {
        top: '50%',
        left: '50%',
        transform: 'translate(-50%, -50%)',
      };
    }

    const rect = spotlightRect();
    const panel = panelRef();
    const desktopWidth = window.innerWidth >= 1024 ? 376 : 344;
    const panelWidth = Math.min(desktopWidth, window.innerWidth - 32);
    const panelHeight = panel?.offsetHeight ?? 260;

    if (!rect) {
      return {
        width: `${panelWidth}px`,
        top: '50%',
        left: '50%',
        transform: 'translate(-50%, -50%)',
      };
    }

    const spaceBelow = window.innerHeight - (rect.top + rect.height);
    const prefersAbove = spaceBelow < panelHeight + 24 && rect.top > panelHeight + 24;
    const unclampedTop = prefersAbove ? rect.top - panelHeight - 20 : rect.top + rect.height + 20;
    const maxTop = Math.max(16, window.innerHeight - panelHeight - 16);
    const top = clamp(unclampedTop, 16, maxTop);
    const unclampedLeft = rect.left + rect.width / 2 - panelWidth / 2;
    const maxLeft = Math.max(16, window.innerWidth - panelWidth - 16);
    const left = clamp(unclampedLeft, 16, maxLeft);

    return {
      width: `${panelWidth}px`,
      top: `${top}px`,
      left: `${left}px`,
    };
  });

  const spotlightStyle = createMemo(() => {
    const rect = spotlightRect();
    if (!rect) return null;

    return {
      top: `${rect.top}px`,
      left: `${rect.left}px`,
      width: `${rect.width}px`,
      height: `${rect.height}px`,
      'box-shadow':
        '0 0 0 9999px rgba(15, 23, 42, 0.78), 0 0 0 2px rgba(255, 255, 255, 0.4), 0 0 40px rgba(96, 165, 250, 0.75)',
    };
  });

  return {
    currentStep,
    dontShowAgain,
    handleClose,
    handleNext,
    handlePrevious,
    handleSelectStep,
    isFirstStep,
    isLastStep,
    isOpen,
    panelStyle,
    setDontShowAgain,
    setPanelRef,
    spotlightStyle,
    stepCount: () => WHATS_NEW_FEATURE_CARDS.length,
    stepIndex,
  };
}
