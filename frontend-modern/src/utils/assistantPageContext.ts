import type { AIChatContext } from '@/stores/aiChat';
import { getActiveTabForPath, type ActiveAppTabId } from '@/routing/navigation';

export interface AssistantPageContextPresentation {
  ariaLabel: string;
  commandDescription: string;
  commandLabel: string;
  context: AIChatContext;
  label: string;
  title: string;
}

const VIEW_LABELS: Record<Exclude<ActiveAppTabId, null>, string> = {
  alerts: 'Alerts',
  ai: 'Patrol',
  docker: 'Docker',
  kubernetes: 'Kubernetes',
  proxmox: 'Proxmox',
  settings: 'Settings',
  standalone: 'Machines',
  truenas: 'TrueNAS',
  vmware: 'vSphere',
};

const normalizePathname = (pathname: string): string => {
  const trimmed = pathname.trim();
  if (!trimmed) return '/';
  return trimmed.startsWith('/') ? trimmed : `/${trimmed}`;
};

export function getAssistantPageContext(pathname: string): AssistantPageContextPresentation {
  const route = normalizePathname(pathname);
  const activeTab = getActiveTabForPath(route);
  const label = activeTab ? VIEW_LABELS[activeTab] : 'this view';
  const contextName = activeTab ? label : 'Current view';
  const subject = activeTab ? `${label} view` : 'Current Pulse view';

  return {
    ariaLabel: `Ask Pulse Assistant about ${label}`,
    commandDescription: activeTab
      ? `Use the current ${label} view as context`
      : 'Use the current Pulse view as context',
    commandLabel: `Ask about ${label}`,
    label,
    title: `Ask Pulse Assistant about ${label}`,
    context: {
      targetType: 'pulse-view',
      targetId: route,
      context: {
        name: contextName,
        route,
        surface: activeTab ?? 'unknown',
      },
      briefing: {
        sourceLabel: 'Current view',
        title: `${contextName} attached`,
        subject,
        statusLabel: 'Context only',
      },
    },
  };
}
