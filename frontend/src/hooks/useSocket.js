import { useState, useEffect, useCallback, useRef } from 'react';
import { io } from 'socket.io-client';
import { clearAppData } from '../utils/storageUtils';

// Add this function for more dynamic mock data generation
const generateDynamicMetric = (baseValue, min, max, changeRange) => {
  // Random value between -changeRange and +changeRange
  const change = (Math.random() * changeRange * 2) - changeRange;
  return Math.max(min, Math.min(max, baseValue + change));
};

/**
 * Custom hook to manage WebSocket connections with Socket.io
 * @param {string} url - WebSocket server URL (defaults to current origin)
 * @returns {Object} Socket state and message handlers
 */
const useSocket = (url) => {
  // In development mode, we need to connect to the backend server on port 7654
  // In production, we can use window.location.origin since both frontend and backend are served from the same origin
  const isDevelopment = import.meta.env.DEV;
  
  // Get the current host
  const currentHost = window.location.hostname;
  
  // For development, we need to explicitly connect to the backend server
  // For production, we use the same origin that served the page
  let socketUrl;
  if (isDevelopment) {
    // First check environment variables, then fall back to localStorage
    const envUseMockData = import.meta.env.VITE_USE_MOCK_DATA === 'true';
    const envMockDataEnabled = import.meta.env.VITE_MOCK_DATA_ENABLED === 'true';
    
    // If environment variables are set, they take precedence
    // Otherwise, check localStorage
    const useMockData = (envUseMockData || envMockDataEnabled) || 
                        (localStorage.getItem('use_mock_data') === 'true' || 
                         localStorage.getItem('MOCK_DATA_ENABLED') === 'true');
    
    // Always connect to the backend server (7654), which will handle the mock data if needed
    socketUrl = `http://${currentHost}:7654`;
  } else {
    // In production, use the same origin that served the page
    socketUrl = window.location.origin;
  }
  
  // Check if we're using mock data - prioritize environment variables
  const envUseMockData = import.meta.env.VITE_USE_MOCK_DATA === 'true';
  const envMockDataEnabled = import.meta.env.VITE_MOCK_DATA_ENABLED === 'true';
  
  // First check environment variables, then fall back to localStorage
  const useMockData = (envUseMockData || envMockDataEnabled) || 
                      (localStorage.getItem('use_mock_data') === 'true' || 
                       localStorage.getItem('MOCK_DATA_ENABLED') === 'true');
  
  // Store the mock data status as a state variable so it can be exposed in the return value
  const [isMockData, setIsMockData] = useState(useMockData);
  
  const [isConnected, setIsConnected] = useState(useMockData ? true : false);
  const [lastMessage, setLastMessage] = useState(null);
  const [nodeData, setNodeData] = useState(useMockData ? [
    { id: 'node-1', name: 'MOCK-pve1', status: 'online' },
    { id: 'node-2', name: 'MOCK-pve2', status: 'online' },
    { id: 'node-3', name: 'MOCK-pve3', status: 'online' }
  ] : []);
  const [guestData, setGuestData] = useState([]);
  const [metricsData, setMetricsData] = useState([]);
  const [processedMetricsData, setProcessedMetricsData] = useState({
    cpu: {},
    memory: {},
    disk: {},
    network: {}
  });
  const [forceUpdateCounter, setForceUpdateCounter] = useState(0);
  const [error, setError] = useState(null);
  const [connectionStatus, setConnectionStatus] = useState(useMockData ? 'connected' : 'connecting');
  
  // Use ref to maintain socket instance across renders
  const socketRef = useRef(null);
  // Use ref for ping interval
  const pingIntervalRef = useRef(null);
  // Track reconnection attempts to prevent infinite loops
  const reconnectAttemptsRef = useRef(0);
  // Reduce reconnection attempts in development mode
  const MAX_RECONNECT_ATTEMPTS = isDevelopment ? 1 : 3;

  // Also add a metrics update interval
  const metricsIntervalRef = useRef(null);
  // Track if we've already set up the interval
  const hasSetupMetricsInterval = useRef(false);

  // Initialize socket connection
  useEffect(() => {
    // Clear any existing socket connection
    if (socketRef.current) {
      socketRef.current.disconnect();
    }
    
    // Log connection attempt
    console.log(`Attempting to connect to socket server at: ${socketUrl}`);
    
    // In mock data mode, we can immediately set up the UI without waiting for a connection
    if (useMockData) {
      console.log('Using mock data mode - setting up immediate UI rendering');
      setIsConnected(true);
      setConnectionStatus('connected');
      
      // Set up mock data
      const mockNodes = [
        { id: 'node-1', name: 'MOCK-pve1', status: 'online' },
        { id: 'node-2', name: 'MOCK-pve2', status: 'online' },
        { id: 'node-3', name: 'MOCK-pve3', status: 'online' }
      ];
      setNodeData(mockNodes);
    }
    
    // Set a timeout to ensure we don't wait forever for the socket connection
    const connectionTimeout = setTimeout(() => {
      if (!isConnected) {
        console.log('Socket connection timeout - using fallback');
        // Force connected state for mock data mode
        if (useMockData || isDevelopment) {
          setIsConnected(true);
          setConnectionStatus('connected');
        }
      }
    }, 3000); // Reduced from 5000ms to 3000ms
    
    try {
      // Create new socket connection
      socketRef.current = io(socketUrl, {
        transports: ['websocket', 'polling'],
        reconnectionAttempts: MAX_RECONNECT_ATTEMPTS,
        reconnectionDelay: 2000,
        reconnectionDelayMax: 10000,
        timeout: 20000,
        autoConnect: true,
        forceNew: true,
        randomizationFactor: 0.5
      });
      
      // Set up event handlers
      socketRef.current.on('connect', () => {
        console.log('Socket connected successfully');
        clearTimeout(connectionTimeout); // Clear the timeout on successful connection
        setIsConnected(true);
        setConnectionStatus('connected');
        setError(null);
        reconnectAttemptsRef.current = 0; // Reset reconnect attempts on successful connection
        
        // Request server configuration to check if mock data is enabled on the server
        socketRef.current.emit('getServerConfig', (config) => {
          if (config && (config.useMockData || config.mockDataEnabled)) {
            console.log('Server is using mock data');
            setIsMockData(true);
            // Store this information in localStorage for persistence
            localStorage.setItem('use_mock_data', 'true');
            localStorage.setItem('MOCK_DATA_ENABLED', 'true');
          } else {
            console.log('Server is using real data');
            setIsMockData(false);
            // Clear the localStorage values
            localStorage.removeItem('use_mock_data');
            localStorage.removeItem('MOCK_DATA_ENABLED');
          }
        });
        
        // Request initial data
        socketRef.current.emit('requestNodeData');
        socketRef.current.emit('requestGuestData');
        socketRef.current.emit('requestMetricsData');
        
        // Start ping interval
        if (pingIntervalRef.current) {
          clearInterval(pingIntervalRef.current);
        }
        pingIntervalRef.current = setInterval(() => {
          if (socketRef.current && socketRef.current.connected) {
            socketRef.current.emit('ping', { timestamp: Date.now() });
          }
        }, 5000);
      });
      
      // Handle page visibility changes
      const handleVisibilityChange = () => {
        if (document.visibilityState === 'visible' && socketRef.current && !socketRef.current.connected) {
          if (reconnectAttemptsRef.current < MAX_RECONNECT_ATTEMPTS) {
            reconnectAttemptsRef.current++;
            reconnect();
          } else {
            console.warn(`Maximum reconnection attempts (${MAX_RECONNECT_ATTEMPTS}) reached. Not attempting further reconnections.`);
            // If using mock data, still show the UI
            if (useMockData) {
              setIsConnected(true);
              setConnectionStatus('connected');
            }
          }
        }
      };
      
      // Handle page unload
      const handleBeforeUnload = () => {
        if (socketRef.current) {
          socketRef.current.disconnect();
        }
      };
      
      // Set up page visibility and unload listeners
      document.addEventListener('visibilitychange', handleVisibilityChange);
      window.addEventListener('beforeunload', handleBeforeUnload);
      
      // Handle connection errors
      socketRef.current.on('connect_error', (err) => {
        console.error('Socket connection error:', err);
        
        // In development mode, immediately fall back to mock data mode
        if (isDevelopment) {
          console.log('Development mode: Immediately falling back to mock data mode');
          socketRef.current.disconnect();
          setConnectionStatus('connected'); // Set to connected anyway to show the UI
          setIsConnected(true);
          
          // Generate mock data for the UI
          const mockNodes = [
            { id: 'node-1', name: 'MOCK-pve1', status: 'online' },
            { id: 'node-2', name: 'MOCK-pve2', status: 'online' },
            { id: 'node-3', name: 'MOCK-pve3', status: 'online' }
          ];
          
          // Set mock data
          setNodeData(mockNodes);
          return;
        }
        
        reconnectAttemptsRef.current++;
        
        if (reconnectAttemptsRef.current >= MAX_RECONNECT_ATTEMPTS) {
          console.warn(`Maximum reconnection attempts (${MAX_RECONNECT_ATTEMPTS}) reached. Not attempting further reconnections.`);
          socketRef.current.disconnect();
          setConnectionStatus('error');
          setIsConnected(false);
          setError(`Connection error after ${MAX_RECONNECT_ATTEMPTS} attempts: ${err.message}`);
          
          // If we're using mock data, we can still show the UI with mock data
          if (useMockData) {
            // Generate some mock data for the UI
            console.log('Using mock data fallback due to connection error');
            
            // Mock node data
            const mockNodes = [
              { id: 'node-1', name: 'MOCK-pve1', status: 'online' },
              { id: 'node-2', name: 'MOCK-pve2', status: 'online' },
              { id: 'node-3', name: 'MOCK-pve3', status: 'online' }
            ];
            
            // Set mock data
            setNodeData(mockNodes);
            
            // Set connection status to connected so the UI renders
            setConnectionStatus('connected');
            setIsConnected(true);
          }
        } else {
          setConnectionStatus('error');
          setIsConnected(false);
          setError(`Connection error: ${err.message}`);
          
          // If we're using mock data, we can still show the UI with mock data
          if (useMockData) {
            // Generate some mock data for the UI
            console.log('Using mock data fallback due to connection error');
            
            // Mock node data
            const mockNodes = [
              { id: 'node-1', name: 'MOCK-pve1', status: 'online' },
              { id: 'node-2', name: 'MOCK-pve2', status: 'online' },
              { id: 'node-3', name: 'MOCK-pve3', status: 'online' }
            ];
            
            // Set mock data
            setNodeData(mockNodes);
            
            // Set connection status to connected so the UI renders
            setConnectionStatus('connected');
            setIsConnected(true);
          }
        }
      });
      
      socketRef.current.on('disconnect', (reason) => {
        console.log(`Socket disconnected: ${reason}`);
        setIsConnected(false);
        setConnectionStatus('disconnected');
        
        // Don't attempt to reconnect if we've reached the maximum attempts
        if (reconnectAttemptsRef.current >= MAX_RECONNECT_ATTEMPTS) {
          console.warn(`Maximum reconnection attempts (${MAX_RECONNECT_ATTEMPTS}) reached. Not attempting further reconnections.`);
          // If using mock data, still show the UI
          if (useMockData) {
            setIsConnected(true);
            setConnectionStatus('connected');
          }
          return;
        }
        
        // Only attempt to reconnect for certain disconnect reasons
        if (reason === 'io server disconnect' || reason === 'transport close') {
          reconnectAttemptsRef.current++;
          // Calculate a delay with exponential backoff and randomization
          const baseDelay = 3000; // Base delay of 3 seconds
          const attempt = reconnectAttemptsRef.current;
          const exponentialDelay = baseDelay * Math.pow(1.5, attempt); // Exponential backoff
          const jitter = Math.random() * 2000; // Random jitter up to 2 seconds
          const reconnectDelay = exponentialDelay + jitter;
          
          console.log(`Will attempt reconnection in ${Math.round(reconnectDelay / 1000)} seconds (attempt ${attempt}/${MAX_RECONNECT_ATTEMPTS})`);
          
          // Attempt to reconnect after the calculated delay
          setTimeout(() => {
            if (socketRef.current) {
              socketRef.current.connect();
            }
          }, reconnectDelay);
        }
      });

      // Message handler
      socketRef.current.on('message', (message) => {
        setLastMessage(message);
        
        switch (message.type) {
          case 'CONNECTED':
            break;
          
          case 'NODE_STATUS_UPDATE':
            handleNodeStatusUpdate(message.payload);
            break;
          
          case 'GUEST_STATUS_UPDATE':
            handleGuestStatusUpdate(message.payload);
            break;
          
          case 'METRICS_UPDATE':
            handleMetricsUpdate(message.payload);
            break;
          
          case 'EVENT':
            handleEventUpdate(message.payload);
            break;
          
          case 'ERROR':
            setError(message.payload);
            break;
          
          default:
        }
      });

      // Direct event handler for migration events
      socketRef.current.on('event', (event) => {
        if (event && event.type === 'migration') {
          handleEventUpdate(event);
        }
      });

      // Setup socket event listeners
      socketRef.current.on('nodeData', (data) => {
        if (Array.isArray(data)) {
          // Check if this is mock data by looking for the 'MOCK-' prefix in node names
          const isMockDataNodes = data.some(node => node.name && node.name.startsWith('MOCK-'));
          if (isMockDataNodes) {
            console.log('Detected mock data from node names');
            setIsMockData(true);
          }
          setNodeData(data);
        }
      });

      // Cleanup function
      return () => {
        clearTimeout(connectionTimeout); // Clear the timeout on cleanup
        if (socketRef.current) {
          socketRef.current.disconnect();
        }
        
        // Remove event listeners
        document.removeEventListener('visibilitychange', handleVisibilityChange);
        window.removeEventListener('beforeunload', handleBeforeUnload);
      };
    } catch (error) {
      clearTimeout(connectionTimeout); // Clear the timeout on error
      console.error('Error initializing socket connection:', error);
      setError('Failed to initialize socket connection');
      
      // If we're using mock data, still show the UI
      if (useMockData) {
        setIsConnected(true);
        setConnectionStatus('connected');
      } else {
        setConnectionStatus('error');
      }
    }
  }, [socketUrl, useMockData]); // Remove isConnected from dependency array to prevent reconnection loops

  // Generate mock guest data if we're using mock data and have no real data
  useEffect(() => {
    if (useMockData && guestData.length === 0 && nodeData.length > 0) {
      console.log('Generating mock guest data');
      
      // Generate some basic mock guest data with more stopped guests
      const mockGuests = [
        { id: '101', name: 'mock-db-master', type: 'vm', status: 'running', node: 'node-1', uptime: 432000 }, // 5 days
        { id: '104', name: 'mock-web1', type: 'vm', status: 'running', node: 'node-1', uptime: 345600 }, // 4 days
        { id: '107', name: 'mock-web2', type: 'vm', status: 'stopped', node: 'node-1', uptime: 0 },
        { id: '110', name: 'mock-redis', type: 'vm', status: 'running', node: 'node-1', uptime: 172800 }, // 2 days
        { id: '121', name: 'mock-dev-db', type: 'vm', status: 'running', node: 'node-2', uptime: 86400 }, // 1 day
        { id: '125', name: 'mock-dev-web1', type: 'vm', status: 'stopped', node: 'node-2', uptime: 0 },
        { id: '126', name: 'mock-dev-web2', type: 'vm', status: 'paused', node: 'node-2', uptime: 0 },
        { id: '130', name: 'mock-test-api', type: 'vm', status: 'stopped', node: 'node-2', uptime: 0 },
        { id: '142', name: 'mock-jenkins', type: 'vm', status: 'running', node: 'node-2', uptime: 259200 }, // 3 days
        { id: '150', name: 'mock-win10-test', type: 'vm', status: 'stopped', node: 'node-3', uptime: 0 },
        { id: '151', name: 'mock-ubuntu22-template', type: 'vm', status: 'stopped', node: 'node-3', uptime: 0 },
        { id: '155', name: 'mock-debian11-template', type: 'vm', status: 'running', node: 'node-3', uptime: 518400 }, // 6 days
        { id: '203', name: 'mock-lb1', type: 'ct', status: 'running', node: 'node-1', uptime: 604800 }, // 7 days
        { id: '204', name: 'mock-lb2', type: 'ct', status: 'stopped', node: 'node-1', uptime: 0 },
        { id: '220', name: 'mock-dev-tools', type: 'ct', status: 'running', node: 'node-2', uptime: 691200 }, // 8 days
        { id: '225', name: 'mock-staging-proxy', type: 'ct', status: 'stopped', node: 'node-2', uptime: 0 },
        { id: '240', name: 'mock-pihole-dns', type: 'ct', status: 'running', node: 'node-3', uptime: 1209600 }, // 14 days
        { id: '245', name: 'mock-home-automation', type: 'ct', status: 'running', node: 'node-3', uptime: 777600 }, // 9 days
        { id: '250', name: 'mock-media-server', type: 'ct', status: 'stopped', node: 'node-3', uptime: 0 },
        { id: '999', name: 'mock-shared-vm', type: 'vm', status: 'running', node: 'node-1', uptime: 259200 }, // 3 days
        { id: '888', name: 'mock-shared-container', type: 'ct', status: 'running', node: 'node-1', uptime: 172800 } // 2 days
      ];
      
      setGuestData(mockGuests);
    }
  }, [useMockData, guestData.length, nodeData]);
  
  // Generate mock metrics data if we're using mock data and have no real metrics
  useEffect(() => {
    if (useMockData && guestData.length > 0) {
      console.log('Setting up mock metrics with %d guests (%d running)', 
        guestData.length, 
        guestData.filter(g => g.status === 'running').length
      );
      
      // Initialize metrics data if empty
      if (metricsData.length === 0) {
        // Create initial mock metrics for running guests
        const mockMetrics = guestData
          .filter(guest => guest.status === 'running')  // Only generate metrics for running guests
          .map(guest => {
            // Generate more interesting initial values with guaranteed minimums
            const cpuUsage = Math.max(25, 10 + Math.random() * 40); // 25-50% initial CPU
            const memPercent = Math.max(30, 20 + Math.random() * 40); // 30-60% initial memory
            const diskPercent = Math.max(40, 30 + Math.random() * 50); // 40-80% initial disk
            
            console.log(`Generating initial metrics for ${guest.name || 'unnamed'} (${guest.id}): CPU: ${cpuUsage.toFixed(1)}%, Memory: ${memPercent.toFixed(1)}%, Disk: ${diskPercent.toFixed(1)}%`);
            
            return {
              guestId: guest.id,
              nodeId: guest.node || 'node-1',
              timestamp: Date.now(),
              metrics: {
                cpu: cpuUsage,
                memory: {
                  total: 16 * 1024 * 1024 * 1024, // 16 GB
                  used: (16 * 1024 * 1024 * 1024) * (memPercent / 100), // Memory used based on percentage
                  percentUsed: memPercent
                },
                disk: {
                  total: 500 * 1024 * 1024 * 1024, // 500 GB
                  used: (500 * 1024 * 1024 * 1024) * (diskPercent / 100), // Disk used based on percentage
                  percentUsed: diskPercent
                },
                network: {
                  inRate: Math.random() * 20 * 1024 * 1024, // 0-20 MB/s
                  outRate: Math.random() * 10 * 1024 * 1024, // 0-10 MB/s
                },
                uptime: guest.uptime || 3600 * (1 + Math.floor(Math.random() * 72)) // Ensure uptime value exists
              }
            };
          });
        
        console.log(`Generated ${mockMetrics.length} initial mock metrics`);
        setMetricsData(mockMetrics);
        
        // Also initialize the processed metrics data
        const processedData = {
          cpu: {},
          memory: {},
          disk: {},
          network: {}
        };
        
        // Process each mock metric entry for UI components
        mockMetrics.forEach(metricEntry => {
          const { guestId, metrics } = metricEntry;
          
          // CPU - ensure it's a number and has correct properties
          const cpuValue = parseFloat(metrics.cpu) || 0;
          processedData.cpu[guestId] = {
            usage: cpuValue
          };
          
          // Memory - ensure percentages are numbers and have correct properties
          const memPercentValue = parseFloat(metrics.memory.percentUsed) || 0;
          processedData.memory[guestId] = {
            total: metrics.memory.total,
            used: metrics.memory.used,
            percentUsed: memPercentValue,
            usagePercent: memPercentValue // Important: UI uses this property
          };
          
          // Disk: More dynamic changes with occasional cleanups
          let diskDelta;
          if (Math.random() < 0.1) {  // Doubled chance from 5% to 10%
            // More significant disk cleanup
            diskDelta = -1 * (Math.random() * 6 + 2); // 2-8% reduction (doubled)
            console.log(`Disk cleanup for ${guestId}: ${metrics.disk.percentUsed.toFixed(1)}% -> ${(metrics.disk.percentUsed + diskDelta).toFixed(1)}%`);
          } else {
            // Disk more noticeable growth
            diskDelta = Math.random() * 1.5; // 0-1.5% growth (tripled from 0.5%)
          }
          const newDiskPercent = Math.max(20, Math.min(95, metrics.disk.percentUsed + diskDelta));
          
          // Network: More dramatic bursty traffic patterns
          let newInRate, newOutRate;
          if (Math.random() < 0.3) {  // Increased from 20% to 30% chance
            // Major traffic burst
            newInRate = Math.max(1024, metrics.network.inRate + (Math.random() * 20 + 5) * 1024 * 1024); // Doubled
            newOutRate = Math.max(1024, metrics.network.outRate + (Math.random() * 10 + 3) * 1024 * 1024); // Doubled
            console.log(`Network burst for ${guestId}: In: ${(metrics.network.inRate/(1024*1024)).toFixed(1)}MB/s -> ${(newInRate/(1024*1024)).toFixed(1)}MB/s`);
          } else {
            // Larger variations
            const inRateChange = (Math.random() * 8 - 4) * 1024 * 1024; // Doubled variation range
            const outRateChange = (Math.random() * 6 - 3) * 1024 * 1024; // Doubled variation range
            newInRate = Math.max(1024, metrics.network.inRate + inRateChange);
            newOutRate = Math.max(1024, metrics.network.outRate + outRateChange);
          }
          
          processedData.disk[guestId] = {
            total: metrics.disk.total,
            used: metrics.disk.used,
            percentUsed: newDiskPercent,
            usagePercent: newDiskPercent // Important: UI uses this property
          };
          
          processedData.network[guestId] = {
            inRate: newInRate,
            outRate: newOutRate
          };
        });
        
        // Update the processed metrics data state
        setProcessedMetricsData(processedData);
      }
      
      // Always set up an interval to update mock metrics, regardless of whether
      // we just initialized the data or not
      console.log("Setting up metrics update interval");
      
      // Only set up the interval if it's not already running
      if (metricsIntervalRef.current) {
        clearInterval(metricsIntervalRef.current);
        metricsIntervalRef.current = null;
      }
      
      // Set a flag to indicate we're setting up the interval
      if (!hasSetupMetricsInterval.current) {
        console.log("First time setting up metrics interval");
        hasSetupMetricsInterval.current = true;
      } else {
        console.log("Re-setting up metrics interval");
      }
      
      metricsIntervalRef.current = setInterval(() => {
        try {
          // Only log every 5th update to reduce console spam
          const shouldLog = Math.random() < 0.1; // Only log about 10% of updates

          if (shouldLog) {
            console.log('Mock update interval - updating data');
          }
          
          // Also occasionally change guest status
          if (Math.random() < 0.05) { // 5% chance per interval
            setGuestData(prevGuests => {
              try {
                // Make a copy of the guests array
                const updatedGuests = [...prevGuests];
                
                // Randomly select a guest to change status
                const randomIndex = Math.floor(Math.random() * updatedGuests.length);
                const guest = updatedGuests[randomIndex];
                
                // Determine new status - with probability favoring current state
                let newStatus;
                if (guest.status === 'running') {
                  // 10% chance a running guest stops
                  newStatus = Math.random() < 0.1 ? 'stopped' : 'running';
                } else {
                  // 20% chance a stopped guest starts
                  newStatus = Math.random() < 0.2 ? 'running' : 'stopped';
                }
                
                // Only update if status changed
                if (newStatus !== guest.status) {
                  updatedGuests[randomIndex] = {
                    ...guest,
                    status: newStatus,
                    // Reset or set uptime accordingly
                    uptime: newStatus === 'running' ? 300 : 0 // New running guests start with 5 min uptime
                  };
                  
                  if (shouldLog) {
                    console.log(`Mock guest ${guest.name || 'unnamed'} (${guest.id}): changed status from ${guest.status} to ${newStatus}`);
                  }
                }
                
                return updatedGuests;
              } catch (error) {
                console.error('Error updating guest status:', error);
                return prevGuests; // Return unchanged if error
              }
            });
          }
          
          // Always update uptime for running guests
          setGuestData(prevGuests => {
            try {
              const updatedGuests = prevGuests.map(guest => {
                if (guest && guest.status === 'running') {
                  // Increase uptime by the interval time (in seconds)
                  return {
                    ...guest,
                    uptime: (guest.uptime || 0) + 2 // Add 2 seconds per interval
                  };
                }
                return guest;
              });
              return updatedGuests;
            } catch (error) {
              console.error('Error updating guest uptime:', error);
              return prevGuests; // Return unchanged if error
            }
          });
          
          // Directly increase force update counter to ensure UI re-renders
          setForceUpdateCounter(prev => (prev + 1) % 10000);
          
          // Update metrics regardless of existing data
          setMetricsData(prev => {
            try {
              const currentMetrics = prev || [];
              
              // Get current list of running guests
              const runningGuestIds = guestData
                .filter(g => g && g.status === 'running')
                .map(g => g.id);
                
              // Update existing metrics
              const updatedMetrics = currentMetrics
                .filter(metric => runningGuestIds.includes(metric.guestId)) // Keep only metrics for running guests
                .map(metric => {
                  // Get the corresponding guest to update uptime
                  const guest = guestData.find(g => g.id === metric.guestId);
                  
                  // CPU: More dynamic changes with occasional spikes
                  let newCpu;
                  if (Math.random() < 0.35) {  // Increased chance of spike from 20% to 35%
                    // Significant spike
                    newCpu = Math.min(95, metric.metrics.cpu + 25 + Math.random() * 30);
                    if (shouldLog) {
                      console.log(`CPU spike for ${metric.guestId}: ${(metric.metrics.cpu || 0).toFixed(1)}% -> ${newCpu.toFixed(1)}%`);
                    }
                  } else if (Math.random() < 0.35) {  // Increased chance of drop from 20% to 35%
                    // Significant drop
                    newCpu = Math.max(5, metric.metrics.cpu - 25 - Math.random() * 20);
                    if (shouldLog) {
                      console.log(`CPU drop for ${metric.guestId}: ${(metric.metrics.cpu || 0).toFixed(1)}% -> ${newCpu.toFixed(1)}%`);
                    }
                  } else {
                    // More noticeable regular changes - increase changeRange from 12 to 20
                    newCpu = generateDynamicMetric(metric.metrics.cpu || 0, 5, 95, 20);
                  }
                  
                  // Memory: More noticeable changes
                  let memoryDelta = (Math.random() * 12) - 6; // -6 to +6 base change (increased)
                  if (newCpu > (metric.metrics.cpu || 0) + 10) {
                    // If CPU spiked up, memory likely increases too
                    memoryDelta += 5;
                  } else if (newCpu < (metric.metrics.cpu || 0) - 10) {
                    // If CPU dropped significantly, memory might decrease too
                    memoryDelta -= 2;
                  }
                  const newMemPercent = Math.max(10, Math.min(90, (metric.metrics.memory?.percentUsed || 50) + memoryDelta));
                  const newMemUsed = (metric.metrics.memory?.total || 16 * 1024 * 1024 * 1024) * (newMemPercent / 100);
                  
                  // Disk: More dynamic changes with occasional cleanups
                  let diskDelta;
                  if (Math.random() < 0.15) {  // Increased chance from 10% to 15%
                    // More significant disk cleanup
                    diskDelta = -1 * (Math.random() * 8 + 3); // 3-11% reduction (increased)
                    if (shouldLog) {
                      console.log(`Disk cleanup for ${metric.guestId}: ${(metric.metrics.disk?.percentUsed || 0).toFixed(1)}% -> ${((metric.metrics.disk?.percentUsed || 50) + diskDelta).toFixed(1)}%`);
                    }
                  } else {
                    // Disk more noticeable growth
                    diskDelta = Math.random() * 2.5; // 0-2.5% growth (increased)
                  }
                  const newDiskPercent = Math.max(20, Math.min(95, (metric.metrics.disk?.percentUsed || 50) + diskDelta));
                  const newDiskUsed = (metric.metrics.disk?.total || 500 * 1024 * 1024 * 1024) * (newDiskPercent / 100);
                  
                  // Network: More dramatic bursty traffic patterns
                  let newInRate, newOutRate;
                  if (Math.random() < 0.4) {  // Increased from 30% to 40% chance
                    // Major traffic burst
                    newInRate = Math.max(1024, (metric.metrics.network?.inRate || 1024) + (Math.random() * 30 + 10) * 1024 * 1024); // Increased
                    newOutRate = Math.max(1024, (metric.metrics.network?.outRate || 1024) + (Math.random() * 15 + 5) * 1024 * 1024); // Increased
                    if (shouldLog) {
                      console.log(`Network burst for ${metric.guestId}: In: ${((metric.metrics.network?.inRate || 0)/(1024*1024)).toFixed(1)}MB/s -> ${(newInRate/(1024*1024)).toFixed(1)}MB/s`);
                    }
                  } else {
                    // Larger variations
                    const inRateChange = (Math.random() * 12 - 6) * 1024 * 1024; // Increased variation range
                    const outRateChange = (Math.random() * 10 - 5) * 1024 * 1024; // Increased variation range
                    newInRate = Math.max(1024, (metric.metrics.network?.inRate || 1024) + inRateChange);
                    newOutRate = Math.max(1024, (metric.metrics.network?.outRate || 1024) + outRateChange);
                  }
                  
                  return {
                    ...metric,
                    timestamp: Date.now(),
                    metrics: {
                      ...metric.metrics,
                      cpu: newCpu,
                      memory: {
                        ...(metric.metrics?.memory || {}),
                        used: newMemUsed,
                        percentUsed: newMemPercent,
                        total: metric.metrics?.memory?.total || 16 * 1024 * 1024 * 1024 // Ensure total is defined
                      },
                      disk: {
                        ...(metric.metrics?.disk || {}),
                        percentUsed: newDiskPercent,
                        used: newDiskUsed,
                        total: metric.metrics?.disk?.total || 500 * 1024 * 1024 * 1024 // Ensure total is defined
                      },
                      network: {
                        ...(metric.metrics?.network || {}),
                        inRate: newInRate,
                        outRate: newOutRate
                      },
                      uptime: guest?.uptime || (metric.metrics?.uptime || 0) + 2 // Ensure uptime is defined
                    }
                  };
                });
              
              // Add metrics for newly running guests
              const existingMetricGuestIds = updatedMetrics.map(m => m.guestId);
              const newRunningGuests = guestData.filter(g => 
                g.status === 'running' && !existingMetricGuestIds.includes(g.id)
              );
              
              // Generate metrics for new running guests
              newRunningGuests.forEach(guest => {
                // Generate highly visible initial values with high minimums and maximums
                const cpuUsage = Math.max(40, 30 + Math.random() * 40); // 40-70% initial CPU
                const memPercent = Math.max(45, 35 + Math.random() * 40); // 45-75% initial memory 
                const diskPercent = Math.max(50, 40 + Math.random() * 40); // 50-80% initial disk
                
                if (shouldLog) {
                  console.log(`Initial metrics for ${guest.name || 'unnamed'} (${guest.id}): CPU: ${cpuUsage.toFixed(1)}%, Memory: ${memPercent.toFixed(1)}%, Disk: ${diskPercent.toFixed(1)}%`);
                }
                
                updatedMetrics.push({
                  guestId: guest.id,
                  nodeId: guest.node || 'node-1',
                  timestamp: Date.now(),
                  metrics: {
                    cpu: cpuUsage,
                    memory: {
                      total: 16 * 1024 * 1024 * 1024, // 16 GB
                      used: (16 * 1024 * 1024 * 1024) * (memPercent / 100),
                      percentUsed: memPercent
                    },
                    disk: {
                      total: 500 * 1024 * 1024 * 1024, // 500 GB
                      used: (500 * 1024 * 1024 * 1024) * (diskPercent / 100),
                      percentUsed: diskPercent
                    },
                    network: {
                      inRate: Math.random() * 10 * 1024 * 1024, // 0-10 MB/s
                      outRate: Math.random() * 5 * 1024 * 1024, // 0-5 MB/s
                    },
                    uptime: guest.uptime || 300 // 5 minutes default
                  }
                });
              });
              
              // Immediately update the processed metrics data with the updated metrics
              // This is critical - we need to use updatedMetrics, not metricsData state variable
              setProcessedMetricsData(prevData => {
                const processedData = {
                  cpu: {},
                  memory: {},
                  disk: {},
                  network: {}
                };
                
                // Process each metric entry using the updated metrics
                updatedMetrics.forEach(metricEntry => {
                  const { guestId, metrics } = metricEntry;
                  
                  // Only include metrics for running guests
                  if (guestData.find(g => g.id === guestId && g.status === 'running')) {
                    // CPU - ensure it's a number
                    const cpuValue = parseFloat(metrics.cpu) || 0;
                    processedData.cpu[guestId] = {
                      usage: cpuValue
                    };
                    
                    // Memory - ensure percentages are numbers
                    const memPercentValue = parseFloat(metrics.memory.percentUsed) || 0;
                    processedData.memory[guestId] = {
                      total: metrics.memory.total,
                      used: metrics.memory.used,
                      percentUsed: memPercentValue,
                      usagePercent: memPercentValue // Include both for compatibility
                    };
                    
                    // Disk - ensure percentages are numbers
                    const diskPercentValue = parseFloat(metrics.disk.percentUsed) || 0;
                    processedData.disk[guestId] = {
                      total: metrics.disk.total,
                      used: metrics.disk.used,
                      percentUsed: diskPercentValue,
                      usagePercent: diskPercentValue // Important: UI uses this property
                    };
                    
                    // Network - ensure rates are numbers
                    const inRateValue = parseFloat(metrics.network.inRate) || 0;
                    const outRateValue = parseFloat(metrics.network.outRate) || 0;
                    processedData.network[guestId] = {
                      inRate: inRateValue,
                      outRate: outRateValue
                    };
                  }
                });
                
                return processedData;
              });
              
              return updatedMetrics;
            } catch (error) {
              console.error('Error updating metrics data:', error);
              return prev; // Return unchanged if error
            }
          }, 2000); // Update every 2 seconds for less frequent updates
        } catch (error) {
          console.error('Error in mock update interval:', error);
          // Don't rethrow - we want the interval to keep running
        }
      }, 2000); // Update every 2 seconds for less frequent updates
    }
    
    return () => {
      if (metricsIntervalRef.current) {
        clearInterval(metricsIntervalRef.current);
        metricsIntervalRef.current = null;
      }
    };
  }, [useMockData, guestData]);

  // Handler for node status updates
  const handleNodeStatusUpdate = useCallback((payload) => {
    // Handle both single node and array of nodes
    const nodeData = Array.isArray(payload) ? payload : [payload];
    
    setNodeData(prevNodes => {
      const updatedNodes = [...prevNodes];
      
      nodeData.forEach(node => {
        const index = updatedNodes.findIndex(n => n.id === node.id);
        if (index >= 0) {
          updatedNodes[index] = node;
        } else {
          updatedNodes.push(node);
        }
      });
      
      return updatedNodes;
    });
  }, []);

  // Handler for guest status updates
  const handleGuestStatusUpdate = useCallback((payload) => {
    // Handle both single guest and array of guests
    const guestData = Array.isArray(payload) ? payload : [payload];
    
    setGuestData(prevGuests => {
      const updatedGuests = [...prevGuests];
      
      guestData.forEach(guest => {
        const index = updatedGuests.findIndex(g => g.id === guest.id);
        if (index >= 0) {
          updatedGuests[index] = guest;
        } else {
          updatedGuests.push(guest);
        }
      });
      
      // Store the current environment with the guest data
      try {
        localStorage.setItem('guest_data_cache', JSON.stringify({
          environment: import.meta.env.MODE || 'development',
          timestamp: Date.now()
        }));
      } catch (error) {
        console.error('Error storing guest data cache info:', error);
      }
      
      return updatedGuests;
    });
  }, []);

  // Handler for metrics updates
  const handleMetricsUpdate = useCallback((payload) => {
    // Handle both single metric and array of metrics
    const metricData = Array.isArray(payload) ? payload : [payload];
    
    // Use functional update to avoid closure issues with stale state
    setMetricsData(prevMetrics => {
      // Performance optimization: Check if we actually have new data before updating
      if (!metricData.length) return prevMetrics;
      
      // Make a new array to ensure React detects the change
      const newMetrics = [...prevMetrics];
      let hasChanges = false;
      
      // Process updates with a Map for O(1) lookups instead of O(n) array searches
      const metricsMap = new Map();
      
      // First create a map of existing metrics for faster lookup
      prevMetrics.forEach(metric => {
        if (metric.guestId) {
          metricsMap.set(metric.guestId, metric);
        }
      });
      
      // Then process the updates
      metricData.forEach(metric => {
        if (!metric.guestId) {
          return; // Skip metrics without a guestId
        }
        
        const existingMetric = metricsMap.get(metric.guestId);
        
        if (existingMetric) {
          // Update existing metric only if the timestamp is newer
          // Increase minimum time threshold to avoid too frequent updates
          const minUpdateInterval = 1000; // milliseconds (increased from 200ms to 1000ms)
          const timeSinceLastUpdate = metric.timestamp - (existingMetric.timestamp || 0);
          
          if (!existingMetric.timestamp || 
              (metric.timestamp >= existingMetric.timestamp && timeSinceLastUpdate >= minUpdateInterval)) {
            // Find the index in the array
            const index = newMetrics.findIndex(m => m.guestId === metric.guestId);
            if (index >= 0) {
              // Check if the metrics have actually changed, with higher thresholds to reduce UI flicker
              const hasSignificantChange = 
                Math.abs((metric.metrics?.cpu || 0) - (existingMetric.metrics?.cpu || 0)) > 1.0 ||
                Math.abs((metric.metrics?.memory?.percentUsed || 0) - (existingMetric.metrics?.memory?.percentUsed || 0)) > 1.0 ||
                Math.abs((metric.metrics?.disk?.percentUsed || 0) - (existingMetric.metrics?.disk?.percentUsed || 0)) > 1.0 ||
                Math.abs((metric.metrics?.network?.inRate || 0) - (existingMetric.metrics?.network?.inRate || 0)) > 2.0 ||
                Math.abs((metric.metrics?.network?.outRate || 0) - (existingMetric.metrics?.network?.outRate || 0)) > 2.0;
              
              // Always update if it's been more than 3 seconds, regardless of change magnitude
              const forceUpdateInterval = 3000; // 3 seconds (increased from 2 seconds)
              const shouldForceUpdate = timeSinceLastUpdate >= forceUpdateInterval;
              
              if (hasSignificantChange || shouldForceUpdate) {
                newMetrics[index] = {
                  ...existingMetric,
                  ...metric,
                  timestamp: metric.timestamp || Date.now()
                };
                hasChanges = true;
              }
            }
          }
        } else {
          // Add new metric
          newMetrics.push({
            ...metric,
            timestamp: metric.timestamp || Date.now()
          });
          hasChanges = true;
        }
      });
      
      // Only update state if there were actual changes
      return hasChanges ? newMetrics : prevMetrics;
    });
  }, []);

  // Handler for event updates
  const handleEventUpdate = useCallback((event) => {
    if (!event) return;
    
    console.log('Event received:', event);
    
    try {
      // Check if this is a migration event
      const isMigrationEvent = 
        (event.type === 'migration') || 
        (event.details?.type === 'migration') ||
        (event.description?.toLowerCase().includes('migration'));
      
      if (isMigrationEvent) {
        console.log('Migration event received:', event);
        
        // Extract migration details from either format
        const details = event.details || {};
        const guestId = details.guestId || event.guestId || '';
        const guestName = details.guestName || event.guestName || '';
        const fromNode = details.fromNode || event.fromNode || '';
        const toNode = details.toNode || event.toNode || event.node || '';
        const timestamp = details.timestamp || event.eventTime || Date.now();
        
        // Emit a custom event that can be listened to by other components
        const migrationEvent = new CustomEvent('proxmox:migration', { 
          detail: {
            guestId,
            guestName,
            fromNode,
            toNode,
            timestamp,
            originalEvent: event
          } 
        });
        window.dispatchEvent(migrationEvent);
        
        // Request fresh guest data after a migration
        if (socketRef.current) {
          socketRef.current.emit('requestGuestData');
        }
      }
      
      // Always set the last message for other event types
      setLastMessage(event);
    } catch (error) {
      console.error('Error handling event update:', error);
    }
  }, []);

  // Function to manually attempt reconnection
  const reconnect = useCallback(() => {
    if (socketRef.current) {
      setConnectionStatus('connecting');
      socketRef.current.connect();
    }
  }, []);

  // Function to subscribe to specific node or guest
  const subscribeToNode = useCallback((nodeId) => {
    if (socketRef.current && isConnected) {
      socketRef.current.emit('subscribe:node', nodeId);
    }
  }, [isConnected]);

  const subscribeToGuest = useCallback((guestId) => {
    if (socketRef.current && isConnected) {
      socketRef.current.emit('subscribe:guest', guestId);
    }
  }, [isConnected]);

  // Function to get historical data
  const getHistory = useCallback((id) => {
    return new Promise((resolve) => {
      if (socketRef.current && isConnected) {
        socketRef.current.emit('get:history', id, (data) => {
          resolve(data);
        });
      } else {
        resolve([]);
      }
    });
  }, [isConnected]);

  return {
    isConnected,
    lastMessage,
    error,
    nodeData,
    guestData,
    metricsData,
    processedMetricsData,
    forceUpdateCounter,
    connectionStatus,
    reconnect,
    subscribeToNode,
    subscribeToGuest,
    getHistory,
    isMockData
  };
};

export default useSocket; 