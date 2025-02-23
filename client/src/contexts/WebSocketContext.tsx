import React, { createContext, useContext, useState, useEffect } from 'react';
import { useWebSocket } from '../hooks/useWebSocket';
import type { Socket } from 'socket.io-client';
import io from 'socket.io-client';
import { useNodes } from '../contexts/NodeContext';

interface WebSocketContextValue {
  socket: Socket | null;
  isConnected: boolean;
  error: Error | null;
  connect: () => void;
  disconnect: () => void;
  emit: (event: string, data: any) => void;
}

const WebSocketContext = createContext<WebSocketContextValue | null>(null);

export const WebSocketProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [socket, setSocket] = useState<Socket | null>(null);
  const { dispatch } = useNodes();

  console.log('WebSocketProvider rendering', { socket });

  useEffect(() => {
    try {
      console.log('Initializing socket connection');
      const serverUrl = import.meta.env.VITE_SERVER_URL || 
        (import.meta.env.DEV ? 'http://localhost:3000' : window.location.origin);
      
      console.log('Connecting to:', serverUrl);
      const newSocket = io(serverUrl, {
        reconnection: true,
        reconnectionAttempts: 5,
        reconnectionDelay: 1000,
        transports: ['websocket', 'polling']
      });
      
      newSocket.on('connect', () => {
        console.log('Socket connected successfully');
        // Request nodes immediately after connection
        newSocket.emit('getNodes');
      });

      newSocket.on('disconnect', (reason) => {
        console.log('Socket disconnected:', reason);
      });

      newSocket.on('connect_error', (error) => {
        console.error('Socket connection error:', error);
      });

      newSocket.on('nodeStatus', (data) => {
        console.log('WebSocketProvider received nodeStatus:', data);
        dispatch({
          type: 'UPDATE_NODE',
          payload: {
            id: data.nodeId,
            name: data.name,
            host: data.host,
            tokenId: data.tokenId,
            status: data.status
          }
        });
      });

      setSocket(newSocket);

      return () => {
        console.log('Cleaning up socket connection');
        newSocket.disconnect();
      };
    } catch (error) {
      console.error('Error initializing WebSocket:', error);
    }
  }, [dispatch]);

  // Add connection status to the context value
  const contextValue = {
    socket,
    isConnected: socket?.connected || false,
    error: null,
    connect: () => socket?.connect(),
    disconnect: () => socket?.disconnect(),
    emit: (event: string, data: any) => socket?.emit(event, data)
  };

  console.log('WebSocketProvider context value:', contextValue);

  return (
    <WebSocketContext.Provider value={contextValue}>
      {children}
    </WebSocketContext.Provider>
  );
};

export const useWebSocketContext = () => {
  const context = useContext(WebSocketContext);
  if (!context) {
    throw new Error('useWebSocketContext must be used within a WebSocketProvider');
  }
  return context;
}; 