import { createSignal } from 'solid-js';

export function useAlertGroupExpansion() {
  const [expandedGroups, setExpandedGroups] = createSignal<Set<string>>(new Set());

  const toggleGroup = (key: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const isGroupExpanded = (key: string) => expandedGroups().has(key);

  return { isGroupExpanded, toggleGroup };
}
