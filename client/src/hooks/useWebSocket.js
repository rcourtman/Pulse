import { useState, useEffect, useRef, useCallback } from 'react';
import { io } from 'socket.io-client';

// Get the base URLs from environment variables or use defaults
const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:3000';

// Create a singleton map to track active connections
const activeConnections = new Map();
// Track components using each connection
const connectionUsers = new Map();
// Reduce logging in production
const isDev = import.meta.env.DEV;

// Counter for failed connection attempts across all connections
const globalConnectionAttempts = {
  count: 0,
  lastFailureTime: 0
};

// Helper function for conditional logging
const log = (message, ...args) => {
  if (isDev) {
    console.log(`[WebSocket] ${message}`, ...args);
  }
};

// Debounce function to limit function calls
const debounce = (fn, delay) => {
  let timer = null;
  return function(...args) {
    clearTimeout(timer);
    timer = setTimeout(() => {
      fn.apply(this, args);
    }, delay);
  };
};

export const useWebSocket = (socketUrl, serverConfig) => {
  const [lastMessage, setLastMessage] = useState(null);
  const [connectionStatus, setConnectionStatus] = useState('disconnected');
  const [reconnectAttempts, setReconnectAttempts] = useState(0);
  const socketRef = useRef(null);
  const messageQueueRef = useRef([]);
  const processingQueueRef = useRef(false);
  const reconnectTimeoutRef = useRef(null);
  const heartbeatIntervalRef = useRef(null);
  const pingTimeoutRef = useRef(null);
  const previousServerConfigRef = useRef(null);
  const connectionIdRef = useRef(null);
  const componentIdRef = useRef(`component-${Math.random().toString(36).substring(2, 9)}`);
  const connectionTimeoutRef = useRef(null);
  const maxReconnectAttempts = 8; // Increased max reconnection attempts

  if (isDev) {
    log('Hook called with URL:', socketUrl);
    log('Hook called with server config:', serverConfig);
  }

  // Generate a unique connection ID for this server
  const getConnectionId = useCallback(() => {
    if (!serverConfig) return null;
    
    if (serverConfig.id && serverConfig.id.startsWith('node-')) {
      return `${socketUrl}:${serverConfig.id}`;
    } else if (serverConfig.url && serverConfig.tokenId) {
      return `${socketUrl}:${serverConfig.url}:${serverConfig.tokenId}`;
    }
    
    return null;
  }, [socketUrl, serverConfig]);

  // Process the message queue with rate limiting
  const processQueue = useCallback(() => {
    if (processingQueueRef.current || messageQueueRef.current.length === 0) {
      return;
    }
    
    processingQueueRef.current = true;
    
    try {
      // Take the most recent message from the queue
      const message = messageQueueRef.current.pop();
      
      // Clear the queue - we only care about the latest data
      messageQueueRef.current = [];
      
      // Update state with the latest message
      setLastMessage(message);
    } finally {
      processingQueueRef.current = false;
    }
  }, []);

  // Debounced version of processQueue to avoid excessive renders
  const debouncedProcessQueue = useCallback(
    debounce(() => {
      processQueue();
    }, 100), // 100ms debounce time
    [processQueue]
  );

  // Add a message to the queue and schedule processing
  const queueMessage = useCallback((message) => {
    messageQueueRef.current.push(message);
    debouncedProcessQueue();
  }, [debouncedProcessQueue]);

  // Heartbeat mechanism to detect dead connections
  const startHeartbeat = useCallback(() => {
    if (heartbeatIntervalRef.current) {
      clearInterval(heartbeatIntervalRef.current);
    }
    
    // Send a ping every 20 seconds
    heartbeatIntervalRef.current = setInterval(() => {
      if (socketRef.current && socketRef.current.connected) {
        log('Sending ping to server');
        
        // Clear previous ping timeout if exists
        if (pingTimeoutRef.current) {
          clearTimeout(pingTimeoutRef.current);
        }
        
        // Set up timeout for ping response
        pingTimeoutRef.current = setTimeout(() => {
          log('Ping timeout - no pong received');
          if (socketRef.current) {
            log('Connection appears dead, disconnecting for reconnect');
            try {
              socketRef.current.disconnect();
              socketRef.current = null;
            } catch (err) {
              // Ignore errors during disconnect
            }
            setConnectionStatus('error');
            attemptReconnect();
          }
        }, 5000); // Wait 5 seconds for a pong response
        
        // Send ping
        try {
          socketRef.current.emit('ping', { timestamp: Date.now() });
        } catch (err) {
          console.error('Error sending ping:', err);
        }
      }
    }, 20000); // 20 second interval
    
    return () => {
      if (heartbeatIntervalRef.current) {
        clearInterval(heartbeatIntervalRef.current);
      }
      if (pingTimeoutRef.current) {
        clearTimeout(pingTimeoutRef.current);
      }
    };
  }, []);

  // Setup ping/pong handlers
  const setupPingPong = useCallback((socket) => {
    // Handle pong responses
    socket.on('pong', (data) => {
      log('Received pong from server', data);
      if (pingTimeoutRef.current) {
        clearTimeout(pingTimeoutRef.current);
        pingTimeoutRef.current = null;
      }
    });
    
    // Optional - also listen for server-initiated pings
    socket.on('ping', () => {
      try {
        socket.emit('pong', { timestamp: Date.now() });
      } catch (err) {
        console.error('Error sending pong response:', err);
      }
    });
  }, []);

  // Function to manually reconnect
  const reconnect = useCallback(() => {
    log('Manual reconnect requested');
    
    if (socketRef.current) {
      log('Closing existing connection for manual reconnect');
      try {
        socketRef.current.disconnect();
      } catch (err) {
        console.error('Error during manual disconnect:', err);
      }
      socketRef.current = null;
    }
    
    setConnectionStatus('disconnected');
    setReconnectAttempts(0);
    
    // Reset global connection attempts on manual reconnect
    globalConnectionAttempts.count = 0;
    globalConnectionAttempts.lastFailureTime = 0;
    
    // Force a re-render to trigger the useEffect
    previousServerConfigRef.current = null;
    
    // Remove from active connections
    const connectionId = connectionIdRef.current;
    if (connectionId && activeConnections.has(connectionId)) {
      log(`Removing connection ${connectionId} from active connections`);
      activeConnections.delete(connectionId);
      connectionUsers.delete(connectionId);
    }
    
    // Clear any existing timeouts
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    
    if (connectionTimeoutRef.current) {
      clearTimeout(connectionTimeoutRef.current);
      connectionTimeoutRef.current = null;
    }
    
    if (heartbeatIntervalRef.current) {
      clearInterval(heartbeatIntervalRef.current);
      heartbeatIntervalRef.current = null;
    }
    
    if (pingTimeoutRef.current) {
      clearTimeout(pingTimeoutRef.current);
      pingTimeoutRef.current = null;
    }
  }, []);

  // Automatic reconnection function with progressive backoff
  const attemptReconnect = useCallback(() => {
    const now = Date.now();
    
    // If it's been more than 2 minutes since the last failure, reset the global counter
    if (now - globalConnectionAttempts.lastFailureTime > 120000) {
      globalConnectionAttempts.count = 0;
    }
    
    // Update global failure tracking
    globalConnectionAttempts.count++;
    globalConnectionAttempts.lastFailureTime = now;
    
    // Use either local or global attempt count, whichever is higher
    const effectiveAttempts = Math.max(reconnectAttempts, globalConnectionAttempts.count);
    
    if (effectiveAttempts < maxReconnectAttempts) {
      // Exponential backoff with jitter
      const baseDelay = Math.min(1000 * Math.pow(1.5, effectiveAttempts), 30000);
      const jitter = Math.random() * 1000; // Add up to 1s of random jitter
      const delay = baseDelay + jitter;
      
      log(`Scheduling reconnect attempt ${reconnectAttempts + 1} in ${Math.round(delay)}ms (global attempts: ${globalConnectionAttempts.count})`);
      
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      
      reconnectTimeoutRef.current = setTimeout(() => {
        log(`Attempting reconnection #${reconnectAttempts + 1}`);
        setReconnectAttempts(prev => prev + 1);
        
        // Force a re-render to trigger the useEffect
        previousServerConfigRef.current = null;
      }, delay);
    } else {
      log(`Maximum reconnection attempts reached, giving up`);
      setConnectionStatus('failed');
      
      // After 30 seconds in the failed state, reset and try again
      reconnectTimeoutRef.current = setTimeout(() => {
        log('Resetting after extended failure period');
        setReconnectAttempts(0);
        globalConnectionAttempts.count = 0;
        previousServerConfigRef.current = null;
        setConnectionStatus('disconnected');
      }, 30000);
    }
  }, [reconnectAttempts]);

  useEffect(() => {
    // Generate connection ID
    const connectionId = getConnectionId();
    connectionIdRef.current = connectionId;
    const componentId = componentIdRef.current;
    
    // Check if server config has changed
    const hasServerConfigChanged = JSON.stringify(previousServerConfigRef.current) !== JSON.stringify(serverConfig);
    
    // If no connection ID can be generated, don't connect
    if (!connectionId) {
      log('No valid connection ID could be generated, not connecting');
      return;
    }
    
    // Register this component as a user of the connection
    if (!connectionUsers.has(connectionId)) {
      connectionUsers.set(connectionId, new Set());
    }
    connectionUsers.get(connectionId).add(componentId);
    
    // Check if there's already an active connection for this server
    if (activeConnections.has(connectionId)) {
      log(`Found existing connection for ${connectionId}`);
      const existingSocket = activeConnections.get(connectionId);
      
      // If the socket is connected, use it
      if (existingSocket.connected) {
        log(`Using existing connected socket for ${connectionId}`);
        socketRef.current = existingSocket;
        setConnectionStatus('connected');
        
        // Set up event listeners for this instance
        const onMessage = (data) => {
          if (isDev) {
            log(`Received message on existing connection ${connectionId}`);
          }
          
          try {
            // Process the data
            let processedData = data;
            if (typeof data === 'string') {
              try {
                processedData = JSON.parse(data);
              } catch (e) {
                if (isDev) log('Message is not JSON, using as-is');
              }
            }
            
            // Add to message queue
            queueMessage(processedData);
          } catch (err) {
            console.error('Error processing message data:', err);
          }
        };
        
        existingSocket.on('data', onMessage);
        existingSocket.on('message', onMessage);
        
        // Set up heartbeat for connection monitoring
        const cleanupHeartbeat = startHeartbeat();
        
        // Clean up event listeners when unmounting
        return () => {
          existingSocket.off('data', onMessage);
          existingSocket.off('message', onMessage);
          cleanupHeartbeat();
          
          // Remove this component from the connection users
          if (connectionUsers.has(connectionId)) {
            connectionUsers.get(connectionId).delete(componentId);
            
            // If no more components are using this connection, clean it up
            if (connectionUsers.get(connectionId).size === 0) {
              log(`No more components using connection ${connectionId}, cleaning up`);
              connectionUsers.delete(connectionId);
              
              if (existingSocket.connected) {
                log(`Disconnecting unused socket for ${connectionId}`);
                existingSocket.disconnect();
                activeConnections.delete(connectionId);
              }
            }
          }
        };
      } else {
        // If the socket exists but is not connected, remove it
        log(`Existing socket for ${connectionId} is not connected, removing it`);
        activeConnections.delete(connectionId);
      }
    }
    
    // Only reconnect if the server config has actually changed or we're forcing a reconnect
    if (!hasServerConfigChanged && socketRef.current && socketRef.current.connected) {
      log('Server config unchanged and socket connected, skipping reconnection');
      return;
    }
    
    previousServerConfigRef.current = serverConfig;

    // Clean up any existing connection and timeout
    if (socketRef.current) {
      log('Closing existing Socket.IO connection');
      try {
        socketRef.current.disconnect();
      } catch (err) {
        console.error('Error during disconnect:', err);
      }
      socketRef.current = null;
    }
    
    if (reconnectTimeoutRef.current) {
      log('Clearing reconnect timeout');
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    
    if (connectionTimeoutRef.current) {
      clearTimeout(connectionTimeoutRef.current);
      connectionTimeoutRef.current = null;
    }
    
    if (heartbeatIntervalRef.current) {
      clearInterval(heartbeatIntervalRef.current);
      heartbeatIntervalRef.current = null;
    }
    
    if (pingTimeoutRef.current) {
      clearTimeout(pingTimeoutRef.current);
      pingTimeoutRef.current = null;
    }

    // Reset state when server config changes
    if (hasServerConfigChanged) {
      setLastMessage(null);
      setConnectionStatus('disconnected');
      setReconnectAttempts(0);
      messageQueueRef.current = [];
    }

    // If no server config is provided, don't connect
    if (!socketUrl || !serverConfig) {
      log('No server config or socket URL provided, not connecting');
      return;
    }

    log('Connecting with config:', serverConfig);

    // Determine the Socket.IO configuration based on the server config
    let socketOptions = {
      path: '/socket.io',
      transports: ['websocket', 'polling'], // Allow polling fallback for more reliability
      reconnection: true,
      reconnectionAttempts: 3,
      reconnectionDelay: 1000,
      reconnectionDelayMax: 5000,
      timeout: 15000, // Increased timeout for more reliability
      autoConnect: true,
      forceNew: reconnectAttempts > 0, // Force a new connection on reconnect attempts
      // Explicitly set retry params
      retries: 3,
      ackTimeout: 10000,
    };
    
    // For pre-configured nodes, connect to the server's WebSocket endpoint
    if (serverConfig && serverConfig.id && serverConfig.id.startsWith('node-')) {
      // Extract the node number from the ID
      const nodeId = serverConfig.id;
      socketOptions.query = { nodeId };
      log(`Connecting to pre-configured node: ${nodeId} at ${socketUrl}`);
    } else if (serverConfig && serverConfig.url && serverConfig.tokenId && serverConfig.token) {
      // For manually added servers, include the credentials in the URL
      const { url, tokenId, token } = serverConfig;
      socketOptions.query = { url: encodeURIComponent(url), tokenId: encodeURIComponent(tokenId), token: encodeURIComponent(token) };
      log(`Connecting to manual server at ${url} via ${socketUrl}`);
    } else {
      console.error('Invalid server configuration:', serverConfig);
      setConnectionStatus('error');
      return;
    }

    // Connect to the Socket.IO server
    const connectWebSocket = () => {
      try {
        log(`Connecting to Socket.IO: ${socketUrl}`);
        setConnectionStatus('connecting');
        
        // Connect to the Socket.IO server with the /api/ws namespace
        const namespaceUrl = socketUrl;
        
        // Create the socket connection immediately
        const socket = io(namespaceUrl, socketOptions);
        
        socketRef.current = socket;
        
        // Add to active connections
        activeConnections.set(connectionId, socket);
        log(`Added connection ${connectionId} to active connections`);
        
        // Set a timeout to detect stalled connections
        connectionTimeoutRef.current = setTimeout(() => {
          if (socket && socket.connected === false) {
            log('Socket.IO connection attempt timed out after 15 seconds');
            setConnectionStatus('error');
            
            // Clean up this socket
            try {
              socket.disconnect();
            } catch (err) {
              // Ignore errors during disconnect
            }
            
            // Attempt to reconnect
            attemptReconnect();
          }
        }, 15000);

        // Set up ping/pong handlers
        setupPingPong(socket);
        
        // Connection events
        socket.on('connect', () => {
          log('Socket.IO connected');
          if (connectionTimeoutRef.current) {
            clearTimeout(connectionTimeoutRef.current);
            connectionTimeoutRef.current = null;
          }
          
          setConnectionStatus('connected');
          setReconnectAttempts(0);
          globalConnectionAttempts.count = 0;
          
          // Start the heartbeat
          startHeartbeat();
          
          // Subscribe to the node
          if (serverConfig.id && serverConfig.id.startsWith('node-')) {
            const nodeId = serverConfig.id;
            log(`Subscribing to node: ${nodeId}`);
            socket.emit('subscribe', { nodeId });
          }
        });
        
        // Specific events for connection status
        socket.on('connecting', (data) => {
          log('Received connecting status', data);
          setConnectionStatus('connecting');
        });
        
        socket.on('connected', (data) => {
          log('Received connected status', data);
          setConnectionStatus('connected');
        });
        
        socket.on('subscribed', (data) => {
          log('Successfully subscribed to node', data);
        });

        // Data handling - process messages with the queue system
        socket.on('data', (data) => {
          log('Received data message');
          queueMessage(data);
        });
        
        socket.on('message', (data) => {
          log('Received general message');
          queueMessage(data);
        });

        socket.on('error', (error) => {
          console.error('Socket.IO error:', error);
          setConnectionStatus('error');
          attemptReconnect();
        });

        socket.on('disconnect', (reason) => {
          if (connectionTimeoutRef.current) {
            clearTimeout(connectionTimeoutRef.current);
            connectionTimeoutRef.current = null;
          }
          
          log(`Socket.IO disconnected: ${reason}`);
          setConnectionStatus('closed');
          
          // Handle specific disconnect reasons
          if (reason === 'io server disconnect') {
            // Server-side disconnection - need to reconnect manually
            log('Server initiated disconnect, will reconnect manually');
            attemptReconnect();
          } else if (reason === 'transport close' || reason === 'transport error') {
            // Transport-level issue (network problem)
            log('Transport-level disconnection, will reconnect');
            attemptReconnect();
          } else if (reason === 'ping timeout') {
            // Server not responding to pings
            log('Ping timeout, server not responding');
            attemptReconnect();
          }
        });
        
        socket.on('connect_error', (error) => {
          console.error('Socket.IO connection error:', error);
          setConnectionStatus('error');
          attemptReconnect();
        });
        
        socket.on('reconnect', (attemptNumber) => {
          log(`Socket.IO reconnected after ${attemptNumber} attempts`);
          setConnectionStatus('connected');
          setReconnectAttempts(0); // Reset our custom counter when Socket.IO reconnects
          globalConnectionAttempts.count = 0;
        });
        
        socket.on('reconnect_attempt', (attemptNumber) => {
          log(`Socket.IO reconnection attempt ${attemptNumber}`);
          setConnectionStatus('connecting');
        });
        
        socket.on('reconnect_error', (error) => {
          log(`Socket.IO reconnection error: ${error}`);
          // Let our custom reconnection logic take over after Socket.IO fails
          attemptReconnect();
        });
        
        socket.on('reconnect_failed', () => {
          log('Socket.IO reconnection failed');
          // Let our custom reconnection take over
          attemptReconnect();
        });
      } catch (error) {
        console.error('Error creating Socket.IO connection:', error);
        setConnectionStatus('error');
        attemptReconnect();
      }
    };
    
    // Start the connection process
    connectWebSocket();
    
    // Clean up on unmount
    return () => {
      log('Cleaning up WebSocket connection');
      
      // Remove this component from the connection users
      if (connectionUsers.has(connectionId)) {
        connectionUsers.get(connectionId).delete(componentId);
        
        // If no more components are using this connection, clean it up
        if (connectionUsers.get(connectionId).size === 0) {
          log(`No more components using connection ${connectionId}, cleaning up`);
          connectionUsers.delete(connectionId);
          
          if (socketRef.current) {
            log(`Disconnecting unused socket for ${connectionId}`);
            try {
              socketRef.current.disconnect();
            } catch (err) {
              // Ignore errors during disconnect
            }
            socketRef.current = null;
            activeConnections.delete(connectionId);
          }
        } else {
          log(`Other components still using connection ${connectionId}, keeping it active`);
        }
      }
      
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }
      
      if (connectionTimeoutRef.current) {
        clearTimeout(connectionTimeoutRef.current);
        connectionTimeoutRef.current = null;
      }
      
      if (heartbeatIntervalRef.current) {
        clearInterval(heartbeatIntervalRef.current);
        heartbeatIntervalRef.current = null;
      }
      
      if (pingTimeoutRef.current) {
        clearTimeout(pingTimeoutRef.current);
        pingTimeoutRef.current = null;
      }
    };
  }, [socketUrl, serverConfig, getConnectionId, reconnectAttempts, attemptReconnect, queueMessage, startHeartbeat, setupPingPong]);
  
  return { lastMessage, connectionStatus, reconnect };
}; 