import { createContext, useContext } from 'solid-js';
import { getGlobalWebSocketStore } from '@/stores/websocket-global';

export type WebSocketStore = ReturnType<typeof getGlobalWebSocketStore>;

export const WebSocketContext = createContext<WebSocketStore>();

export const useWebSocket = () => {
  const context = useContext(WebSocketContext);
  if (!context) {
    throw new Error('useWebSocket must be used within WebSocketContext.Provider');
  }
  return context;
};

export const DarkModeContext = createContext<() => boolean>();

export const useDarkMode = () => {
  const context = useContext(DarkModeContext);
  if (!context) {
    throw new Error('useDarkMode must be used within DarkModeContext.Provider');
  }
  return context;
};
