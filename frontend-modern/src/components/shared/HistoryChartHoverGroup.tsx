import {
  createContext,
  createSignal,
  useContext,
  type Accessor,
  type ParentComponent,
} from 'solid-js';

export interface HistoryChartHoverGroupState {
  hoveredTimestamp: Accessor<number | null>;
  setHoveredTimestamp: (timestamp: number | null) => void;
}

const HistoryChartHoverContext = createContext<HistoryChartHoverGroupState>();

export const HistoryChartHoverGroup: ParentComponent = (props) => {
  const [hoveredTimestamp, setHoveredTimestamp] = createSignal<number | null>(null);

  return (
    <HistoryChartHoverContext.Provider value={{ hoveredTimestamp, setHoveredTimestamp }}>
      {props.children}
    </HistoryChartHoverContext.Provider>
  );
};

export function useHistoryChartHoverGroup() {
  return useContext(HistoryChartHoverContext);
}
