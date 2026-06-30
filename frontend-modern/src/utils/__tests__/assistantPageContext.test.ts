import { describe, expect, it } from 'vitest';
import { getAssistantPageContext } from '@/utils/assistantPageContext';

describe('assistantPageContext', () => {
  it('attaches monitor-first platform route context to Assistant launches', () => {
    const presentation = getAssistantPageContext('/proxmox/overview');

    expect(presentation).toMatchObject({
      ariaLabel: 'Ask Pulse Assistant about Proxmox',
      commandDescription: 'Use the current Proxmox view as context',
      commandLabel: 'Ask about Proxmox',
      label: 'Proxmox',
      title: 'Ask Pulse Assistant about Proxmox',
    });
    expect(presentation.context).toMatchObject({
      targetType: 'pulse-view',
      targetId: '/proxmox/overview',
      context: {
        name: 'Proxmox',
        route: '/proxmox/overview',
        surface: 'proxmox',
      },
      briefing: {
        sourceLabel: 'Current view',
        title: 'Proxmox attached',
        subject: 'Proxmox view',
        statusLabel: 'Context only',
      },
    });
  });

  it('keeps Patrol as the visible checking-loop surface', () => {
    const presentation = getAssistantPageContext('/patrol');

    expect(presentation.commandLabel).toBe('Ask about Patrol');
    expect(presentation.context.context).toMatchObject({
      name: 'Patrol',
      route: '/patrol',
      surface: 'ai',
    });
    expect(presentation.context.briefing?.title).toBe('Patrol attached');
  });

  it('falls back to current-view context for unowned routes', () => {
    const presentation = getAssistantPageContext('custom/report');

    expect(presentation.ariaLabel).toBe('Ask Pulse Assistant about this view');
    expect(presentation.commandLabel).toBe('Ask about this view');
    expect(presentation.context.targetId).toBe('/custom/report');
    expect(presentation.context.context).toMatchObject({
      name: 'Current view',
      route: '/custom/report',
      surface: 'unknown',
    });
  });

  it('does not revive retired dashboard or Explore surfaces as Assistant context', () => {
    const retiredRoutes = ['/dashboard', '/dashboard/explore', '/explore', '/home'];

    for (const route of retiredRoutes) {
      const presentation = getAssistantPageContext(route);

      expect(presentation.ariaLabel).toBe('Ask Pulse Assistant about this view');
      expect(presentation.commandLabel).toBe('Ask about this view');
      expect(presentation.context.targetId).toBe(route);
      expect(presentation.context.context).toMatchObject({
        name: 'Current view',
        route,
        surface: 'unknown',
      });
      expect(presentation.context.briefing?.title).toBe('Current view attached');
    }
  });
});
