import { createMemo } from 'solid-js';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import {
  DASHBOARD_WIDGETS,
  getDashboardWidget,
  type DashboardWidgetDef,
  type DashboardWidgetId,
} from '@/pages/DashboardPanels/dashboardWidgets';

type DashboardLayoutState = {
  order: DashboardWidgetId[];
  hidden: DashboardWidgetId[];
};

const DASHBOARD_WIDGET_LAYOUT_STORAGE_KEY = 'dashboardWidgetLayout_v2';
const DEFAULT_ORDER = [...DASHBOARD_WIDGETS]
  .sort((a, b) => a.defaultOrder - b.defaultOrder)
  .map((widget) => widget.id);
const DEFAULT_HIDDEN = DASHBOARD_WIDGETS
  .filter((widget) => !widget.defaultVisible)
  .map((widget) => widget.id);
const WIDGET_IDS = new Set<DashboardWidgetId>(DASHBOARD_WIDGETS.map((widget) => widget.id));
const DEFAULT_LAYOUT: DashboardLayoutState = {
  order: [...DEFAULT_ORDER],
  hidden: [...DEFAULT_HIDDEN],
};

function normalizeLayout(layout: DashboardLayoutState): DashboardLayoutState {
  const order = layout.order.filter((id): id is DashboardWidgetId => WIDGET_IDS.has(id));
  const hidden = layout.hidden.filter((id): id is DashboardWidgetId => WIDGET_IDS.has(id));

  for (const id of DEFAULT_ORDER) {
    if (!order.includes(id)) {
      order.push(id);
    }
  }

  return { order, hidden };
}

function parseLayout(raw: string): DashboardLayoutState {
  try {
    const parsed = JSON.parse(raw) as Partial<DashboardLayoutState>;
    if (Array.isArray(parsed.order) && Array.isArray(parsed.hidden)) {
      return normalizeLayout({
        order: parsed.order as DashboardWidgetId[],
        hidden: parsed.hidden as DashboardWidgetId[],
      });
    }
  } catch {
    // Fall through to defaults.
  }

  return DEFAULT_LAYOUT;
}

function arrayEquals<T>(left: T[], right: T[]): boolean {
  if (left.length !== right.length) return false;
  for (let i = 0; i < left.length; i += 1) {
    if (left[i] !== right[i]) return false;
  }
  return true;
}

export function useDashboardLayout() {
  const [layout, setLayout] = usePersistentSignal<DashboardLayoutState>(
    DASHBOARD_WIDGET_LAYOUT_STORAGE_KEY,
    DEFAULT_LAYOUT,
    {
      serialize: (value) => JSON.stringify(value),
      deserialize: parseLayout,
    },
  );

  const visibleWidgets = createMemo<DashboardWidgetDef[]>(() => {
    const hidden = new Set(layout().hidden);
    return layout().order
      .filter((id) => !hidden.has(id))
      .map((id) => getDashboardWidget(id))
      .filter((widget): widget is DashboardWidgetDef => widget !== undefined);
  });

  const allWidgetsOrdered = createMemo<DashboardWidgetDef[]>(() =>
    layout().order
      .map((id) => getDashboardWidget(id))
      .filter((widget): widget is DashboardWidgetDef => widget !== undefined),
  );

  const isHidden = (id: DashboardWidgetId) => layout().hidden.includes(id);

  const toggleWidget = (id: DashboardWidgetId) => {
    setLayout((current) => {
      const hidden = current.hidden.includes(id)
        ? current.hidden.filter((entry) => entry !== id)
        : [...current.hidden, id];
      return { ...current, hidden };
    });
  };

  const moveUp = (id: DashboardWidgetId) => {
    setLayout((current) => {
      const order = [...current.order];
      const idx = order.indexOf(id);
      if (idx <= 0) return current;
      [order[idx - 1], order[idx]] = [order[idx], order[idx - 1]];
      return { ...current, order };
    });
  };

  const moveDown = (id: DashboardWidgetId) => {
    setLayout((current) => {
      const order = [...current.order];
      const idx = order.indexOf(id);
      if (idx < 0 || idx >= order.length - 1) return current;
      [order[idx], order[idx + 1]] = [order[idx + 1], order[idx]];
      return { ...current, order };
    });
  };

  const resetToDefaults = () => {
    setLayout({
      order: [...DEFAULT_ORDER],
      hidden: [...DEFAULT_HIDDEN],
    });
  };

  const isDefault = createMemo(() => {
    const current = layout();
    return arrayEquals(current.order, DEFAULT_ORDER) && arrayEquals(current.hidden, DEFAULT_HIDDEN);
  });

  return {
    visibleWidgets,
    allWidgetsOrdered,
    isHidden,
    toggleWidget,
    moveUp,
    moveDown,
    resetToDefaults,
    isDefault,
  };
}
