import React, { useReducer, useEffect, useMemo, useCallback, useRef } from 'react';
import { useServersStore } from '../contexts/ServersStore';
import { useWebSocket } from '../hooks/useWebSocket';
import useConfiguredNodes from '../hooks/useConfiguredNodes';
import ResourceGrid from '../components/dashboard/ResourceGrid';
import ServerSelector from '../components/dashboard/ServerSelector';
import FilterModal from '../components/dashboard/FilterModal';
import { FaExclamationTriangle, FaSpinner, FaSync } from 'react-icons/fa';
import { API_BASE_URL } from '../config';
import '../styles/dashboard.css';

// Determine if we're in development mode
const isDev = import.meta.env.DEV;

// Error types that are considered transient (will auto-retry)
const TRANSIENT_ERRORS = [
  'transport close',
  'transport error',
  'ping timeout',
  'network error',
  'timeout'
];

// Initial state for the reducer
const initialState = {
  selectedServerId: null,
  resources: [],
  isLoading: true,
  error: null,
  errorDetails: null,
  errorType: null,
  sortConfig: { key: 'cpu', direction: 'desc' },
  searchTerm: '',
  filterTags: [],
  showFilterModal: false,
  hasSetInitialServer: false,
  lastDataTimestamp: null,
  reconnecting: false
};

// Action types
const ActionTypes = {
  SET_SERVER: 'SET_SERVER',
  SET_RESOURCES: 'SET_RESOURCES',
  SET_LOADING: 'SET_LOADING',
  SET_ERROR: 'SET_ERROR',
  SET_SORT_CONFIG: 'SET_SORT_CONFIG',
  SET_SEARCH_TERM: 'SET_SEARCH_TERM',
  SET_FILTER_TAGS: 'SET_FILTER_TAGS',
  TOGGLE_FILTER_MODAL: 'TOGGLE_FILTER_MODAL',
  SET_INITIAL_SERVER: 'SET_INITIAL_SERVER',
  SET_RECONNECTING: 'SET_RECONNECTING',
  CLEAR_ERROR: 'CLEAR_ERROR'
};

// Reducer function
function dashboardReducer(state, action) {
  switch (action.type) {
    case ActionTypes.SET_SERVER:
      return {
        ...state,
        selectedServerId: action.payload,
        resources: [],
        isLoading: true,
        error: null,
        errorDetails: null,
        errorType: null,
        lastDataTimestamp: null
      };
    case ActionTypes.SET_RESOURCES:
      return {
        ...state,
        resources: action.payload,
        isLoading: false,
        error: null,
        errorDetails: null, 
        errorType: null,
        lastDataTimestamp: Date.now()
      };
    case ActionTypes.SET_LOADING:
      return {
        ...state,
        isLoading: action.payload
      };
    case ActionTypes.SET_ERROR:
      return {
        ...state,
        error: action.payload.message,
        errorDetails: action.payload.details || null,
        errorType: action.payload.type || 'unknown',
        isLoading: false
      };
    case ActionTypes.CLEAR_ERROR:
      return {
        ...state,
        error: null,
        errorDetails: null,
        errorType: null
      };
    case ActionTypes.SET_SORT_CONFIG:
      return {
        ...state,
        sortConfig: action.payload
      };
    case ActionTypes.SET_SEARCH_TERM:
      return {
        ...state,
        searchTerm: action.payload
      };
    case ActionTypes.SET_FILTER_TAGS:
      return {
        ...state,
        filterTags: action.payload
      };
    case ActionTypes.TOGGLE_FILTER_MODAL:
      return {
        ...state,
        showFilterModal: action.payload
      };
    case ActionTypes.SET_INITIAL_SERVER:
      return {
        ...state,
        selectedServerId: action.payload,
        hasSetInitialServer: true
      };
    case ActionTypes.SET_RECONNECTING:
      return {
        ...state,
        reconnecting: action.payload
      };
    default:
      return state;
  }
}

// Helper function to determine if an error is transient
const isTransientError = (errorType) => {
  if (!errorType) return true;
  return TRANSIENT_ERRORS.some(type => errorType.toLowerCase().includes(type.toLowerCase()));
};

// Helper function to parse network values
const parseNetworkValue = (value) => {
  if (value === undefined || value === null) return 0;
  
  // Handle numeric values directly
  if (typeof value === 'number') {
    return value;
  }
  
  // Handle string values
  if (typeof value === 'string') {
    // Special case for Proxmox "0 B" format
    if (value === '0 B' || value === '0B') return 0;
    
    // Remove any non-numeric characters except decimal point
    const cleanedValue = value.replace(/[^0-9.]/g, '');
    // Try to parse string as number
    const parsed = parseFloat(cleanedValue);
    
    // Check for unit multipliers (K, M, G, T)
    let multiplier = 1;
    const upperValue = value.toUpperCase();
    if (upperValue.includes('K') || upperValue.includes('KB')) multiplier = 1024;
    if (upperValue.includes('M') || upperValue.includes('MB')) multiplier = 1024 * 1024;
    if (upperValue.includes('G') || upperValue.includes('GB')) multiplier = 1024 * 1024 * 1024;
    if (upperValue.includes('T') || upperValue.includes('TB')) multiplier = 1024 * 1024 * 1024 * 1024;
    
    return isNaN(parsed) ? 0 : parsed * multiplier;
  }
  
  // Handle object values
  if (typeof value === 'object' && value !== null) {
    // Arrays (sometimes used in Proxmox)
    if (Array.isArray(value)) {
      return value.length > 0 ? parseNetworkValue(value[0]) : 0;
    }
    
    // Try common properties that might hold the actual value
    if (value.value !== undefined) return parseNetworkValue(value.value);
    if (value.bytes !== undefined) return parseNetworkValue(value.bytes);
    if (value.bps !== undefined) return parseNetworkValue(value.bps);
    if (value.rate !== undefined) return parseNetworkValue(value.rate);
    
    // Proxmox-specific: traffic object might have in/out or tx/rx
    if (value.in !== undefined) return parseNetworkValue(value.in);
    if (value.out !== undefined) return parseNetworkValue(value.out);
    if (value.rx !== undefined) return parseNetworkValue(value.rx);
    if (value.tx !== undefined) return parseNetworkValue(value.tx);
    
    // If no values found, try to find any numeric property
    for (const key in value) {
      if (typeof value[key] === 'number') {
        return value[key];
      }
      if (typeof value[key] === 'string') {
        const parsed = parseFloat(value[key]);
        if (!isNaN(parsed)) {
          return parsed;
        }
      }
    }
  }
  
  return 0;
};

const Dashboard = () => {
  // Use reducer instead of multiple useState calls
  const [state, dispatch] = useReducer(dashboardReducer, initialState);
  const {
    selectedServerId,
    resources,
    isLoading,
    error,
    errorDetails,
    errorType,
    sortConfig,
    searchTerm,
    filterTags,
    showFilterModal,
    hasSetInitialServer,
    reconnecting,
    lastDataTimestamp
  } = state;
  
  // Refs to track message processing
  const lastProcessedMessageRef = useRef(null);
  const errorTimeoutRef = useRef(null);
  
  // Auto reconnect for transient errors
  useEffect(() => {
    if (error && isTransientError(errorType) && !reconnecting) {
      // For transient errors, attempt reconnection after a delay
      if (errorTimeoutRef.current) {
        clearTimeout(errorTimeoutRef.current);
      }
      
      errorTimeoutRef.current = setTimeout(() => {
        if (isDev) {
          console.log(`Auto-reconnecting due to transient error: ${errorType}`);
        }
        
        dispatch({ type: ActionTypes.SET_RECONNECTING, payload: true });
        dispatch({ type: ActionTypes.CLEAR_ERROR });
        dispatch({ type: ActionTypes.SET_LOADING, payload: true });
        
        // Schedule actual reconnect with WebSocket hook
        errorTimeoutRef.current = setTimeout(() => {
          reconnect();
          
          // Clear reconnecting flag after a delay
          setTimeout(() => {
            dispatch({ type: ActionTypes.SET_RECONNECTING, payload: false });
          }, 5000);
        }, 1000);
      }, 5000);
      
      return () => {
        if (errorTimeoutRef.current) {
          clearTimeout(errorTimeoutRef.current);
        }
      };
    }
  }, [error, errorType, reconnecting]);
  
  // Get pre-configured nodes and servers
  const { nodes: configuredNodes, isLoading: isLoadingNodes } = useConfiguredNodes();
  const { servers } = useServersStore();
  
  // Combine all servers
  const allServers = useMemo(() => {
    return [...(configuredNodes || []), ...(servers || [])];
  }, [configuredNodes, servers]);
  
  // Get the selected server
  const selectedServer = useMemo(() => {
    return allServers.find(server => server.id === selectedServerId) || null;
  }, [allServers, selectedServerId]);
  
  // Set the initial server when servers are loaded
  useEffect(() => {
    if (allServers.length > 0 && !selectedServerId && !hasSetInitialServer) {
      dispatch({ 
        type: ActionTypes.SET_INITIAL_SERVER, 
        payload: allServers[0].id 
      });
    }
  }, [allServers, selectedServerId, hasSetInitialServer]);
  
  // Get the WebSocket connection
  const { lastMessage, connectionStatus, reconnect } = useWebSocket(
    selectedServer ? `${API_BASE_URL}/api/ws` : null,
    selectedServer
  );
  
  // Process WebSocket messages with protection against outdated messages
  useEffect(() => {
    // Skip if no message
    if (!lastMessage) {
      return;
    }
    
    try {
      // Extract resources from the message
      const extractedResources = lastMessage.resources || [];
      
      if (extractedResources.length === 0) {
        return;
      }
      
      // Process the resources efficiently
      const processedResources = extractedResources.map(resource => {
        // Create a new object to avoid modifying the original
        const processed = { ...resource };
        
        // Ensure CPU percentage is valid
        if (typeof processed.cpu === 'number') {
          // Determine if this resource reports direct percentages based on properties
          // rather than hardcoding specific resource names
          const isDirect = 
            // Check if the resource is explicitly marked as using direct percentages
            processed.isDirectPercentage === true || 
            // Or if it's specifically tagged as a direct percentage reporter in type
            (processed.type && processed.type.toLowerCase().includes('direct')) ||
            // Or if it has a custom property indicating direct percentage
            processed.directValues === true;
          
          if (isDirect) {
            // For direct percentage reporters, use the value as is
            processed.isDirectPercentage = true;
            processed.cpuPercentage = processed.cpu;
          } else {
            // The server has already converted Proxmox CPU values from 0-1 to 0-100
            // Just use the value directly
            processed.cpuPercentage = processed.cpu;
            if (isDev) {
              console.log(`CPU value for ${processed.name}: ${processed.cpu}%`);
            }
          }
        } else {
          processed.cpuPercentage = 0;
        }
        
        // Process memory values
        if (processed.mem !== undefined && processed.maxmem && processed.maxmem > 0) {
          processed.memoryPercentage = Math.min((processed.mem / processed.maxmem) * 100, 100);
        } else if (processed.memory !== undefined) {
          processed.memoryPercentage = processed.memory;
        } else {
          processed.memoryPercentage = 0;
        }
        
        // Ensure disk percentage is valid
        if (processed.maxdisk && processed.maxdisk > 0 && processed.disk !== undefined) {
          processed.diskPercentage = Math.min((processed.disk / processed.maxdisk) * 100, 100);
        } else if (typeof processed.disk === 'number') {
          processed.diskPercentage = processed.disk;
        } else {
          processed.diskPercentage = 0;
        }
        
        // Add a timestamp for this update
        processed._updateTimestamp = lastMessage.timestamp || Date.now();
        
        return processed;
      });
      
      // Update resources state
      dispatch({
        type: ActionTypes.SET_RESOURCES,
        payload: processedResources
      });
      
    } catch (error) {
      console.error('Error processing WebSocket message:', error);
      dispatch({
        type: ActionTypes.SET_ERROR,
        payload: {
          message: `Error processing data: ${error.message}`,
          details: error.stack,
          type: 'processing_error'
        }
      });
    }
  }, [lastMessage]);
  
  // Handle connection status changes
  const prevConnectionStatusRef = useRef(null);
  
  useEffect(() => {
    if (prevConnectionStatusRef.current === connectionStatus) {
      return;
    }
    
    prevConnectionStatusRef.current = connectionStatus;
    
    if (connectionStatus === 'connecting') {
      dispatch({ type: ActionTypes.SET_LOADING, payload: true });
      dispatch({ type: ActionTypes.CLEAR_ERROR });
    } else if (connectionStatus === 'error') {
      dispatch({ 
        type: ActionTypes.SET_ERROR, 
        payload: {
          message: 'Failed to connect to server',
          type: 'connection_error'
        }
      });
    } else if (connectionStatus === 'failed') {
      dispatch({ 
        type: ActionTypes.SET_ERROR, 
        payload: {
          message: 'Connection failed after multiple attempts',
          type: 'connection_failed'
        }
      });
    } else if (connectionStatus === 'closed' && resources.length === 0) {
      dispatch({ 
        type: ActionTypes.SET_ERROR, 
        payload: {
          message: 'Connection to server closed',
          type: 'connection_closed'
        }
      });
    } else if (connectionStatus === 'connected') {
      // Clear any error when we successfully connect
      dispatch({ type: ActionTypes.CLEAR_ERROR });
      // Keep loading state until we get data or timeout
    }
  }, [connectionStatus, resources.length]);
  
  // Set up a timeout for data reception
  useEffect(() => {
    if (connectionStatus !== 'connected' || resources.length > 0 || !selectedServer) {
      return;
    }
    
    const timer = setTimeout(() => {
      if (resources.length === 0) {
        dispatch({ 
          type: ActionTypes.SET_ERROR, 
          payload: {
            message: 'Connected to server but no data received',
            type: 'no_data_received'
          }
        });
      }
    }, 10000);
    
    return () => clearTimeout(timer);
  }, [connectionStatus, resources.length, selectedServer]);
  
  // Filter and sort resources
  const filteredAndSortedResources = useMemo(() => {
    if (!resources || resources.length === 0) {
      return [];
    }
    
    let result = [...resources];
    
    // Filter for only VM and LXC resources
    result = result.filter(resource => {
      const type = resource.type ? resource.type.toLowerCase() : '';
      return type === 'vm' || type === 'lxc' || type === 'container' || type === 'qemu';
    });
    
    // Apply search term filter
    if (searchTerm) {
      const term = searchTerm.toLowerCase();
      result = result.filter(resource => 
        (resource.name && resource.name.toLowerCase().includes(term)) || 
        (resource.type && resource.type.toLowerCase().includes(term)) ||
        (resource.status && resource.status.toLowerCase().includes(term))
      );
    }
    
    // Apply tag filters
    if (filterTags.length > 0) {
      result = result.filter(resource => {
        return filterTags.some(tag => 
          resource.name && resource.name.toLowerCase().includes(tag.toLowerCase())
        );
      });
    }
    
    // Apply sorting
    if (sortConfig.key) {
      result.sort((a, b) => {
        // Handle special cases for percentage fields
        let aValue, bValue;
        
        if (sortConfig.key === 'cpu') {
          // Use cpuPercentage for consistent comparison (all values in 0-100 range)
          aValue = a.cpuPercentage !== undefined ? a.cpuPercentage : a.cpu;
          bValue = b.cpuPercentage !== undefined ? b.cpuPercentage : b.cpu;
        } else if (sortConfig.key === 'memory') {
          aValue = a.memoryPercentage !== undefined ? a.memoryPercentage : a.memory;
          bValue = b.memoryPercentage !== undefined ? b.memoryPercentage : b.memory;
        } else if (sortConfig.key === 'disk') {
          aValue = a.diskPercentage !== undefined ? a.diskPercentage : a.disk;
          bValue = b.diskPercentage !== undefined ? b.diskPercentage : b.disk;
        } else {
          // Get values, handling different data types appropriately
          aValue = a[sortConfig.key];
          bValue = b[sortConfig.key];
        }
        
        // Convert to numbers for numeric fields
        if (sortConfig.key === 'cpu' || sortConfig.key === 'memory' || sortConfig.key === 'disk' || 
            sortConfig.key === 'networkIn' || sortConfig.key === 'networkOut') {
          aValue = parseFloat(aValue) || 0;
          bValue = parseFloat(bValue) || 0;
        } 
        // Handle string comparisons
        else if (typeof aValue === 'string' && typeof bValue === 'string') {
          return sortConfig.direction === 'asc' 
            ? aValue.localeCompare(bValue)
            : bValue.localeCompare(aValue);
        }
        // Handle undefined or null values
        else {
          aValue = aValue ?? 0;
          bValue = bValue ?? 0;
        }
        
        // Sort based on direction
        return sortConfig.direction === 'asc' 
          ? aValue - bValue 
          : bValue - aValue;
      });
    }
    
    return result;
  }, [resources, searchTerm, filterTags, sortConfig]);
  
  // Event handlers
  const handleServerChange = useCallback((serverId) => {
    dispatch({ type: ActionTypes.SET_SERVER, payload: serverId });
    lastProcessedMessageRef.current = null;
  }, []);
  
  const handleReconnect = useCallback(() => {
    dispatch({ type: ActionTypes.SET_LOADING, payload: true });
    dispatch({ type: ActionTypes.CLEAR_ERROR });
    lastProcessedMessageRef.current = null;
    reconnect();
  }, [reconnect]);
  
  const handleResourceClick = useCallback((resource) => {
    console.log('Resource clicked:', resource);
  }, []);
  
  const handleSort = useCallback((key) => {
    dispatch({ 
      type: ActionTypes.SET_SORT_CONFIG, 
      payload: { 
        key, 
        direction: sortConfig.key === key && sortConfig.direction === 'desc' ? 'asc' : 'desc' 
      } 
    });
  }, [sortConfig.key, sortConfig.direction]);
  
  const handleOpenFilterModal = useCallback(() => {
    dispatch({ type: ActionTypes.TOGGLE_FILTER_MODAL, payload: true });
  }, []);
  
  const handleCloseFilterModal = useCallback(() => {
    dispatch({ type: ActionTypes.TOGGLE_FILTER_MODAL, payload: false });
  }, []);
  
  const handleApplyFilters = useCallback((tags) => {
    dispatch({ type: ActionTypes.SET_FILTER_TAGS, payload: tags });
  }, []);
  
  const handleSearchChange = useCallback((e) => {
    dispatch({ type: ActionTypes.SET_SEARCH_TERM, payload: e.target.value });
  }, []);
  
  // Render loading state for nodes
  if (isLoadingNodes) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen p-4">
        <div className="flex items-center space-x-2 mb-4">
          <FaSpinner className="animate-spin text-primary-600 text-2xl" />
          <span className="text-xl font-semibold">Loading servers...</span>
        </div>
      </div>
    );
  }
  
  // Render main dashboard
  return (
    <div className="container mx-auto p-4">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center mb-6">
        <h1 className="text-2xl font-bold text-gray-800 dark:text-white mb-4 md:mb-0">Dashboard</h1>
        
        <div className="flex flex-col sm:flex-row space-y-4 sm:space-y-0 sm:space-x-4 w-full md:w-auto">
          <ServerSelector 
            servers={allServers} 
            selectedServerId={selectedServerId} 
            onServerChange={handleServerChange} 
            connectionStatus={connectionStatus}
          />
          
          {(connectionStatus === 'error' || connectionStatus === 'closed' || connectionStatus === 'failed') && (
            <button
              onClick={handleReconnect}
              className="flex items-center gap-2 px-3 py-2 bg-primary-500 text-white rounded hover:bg-primary-600 transition-colors"
              disabled={reconnecting}
            >
              {reconnecting ? (
                <>
                  <FaSpinner className="animate-spin" />
                  <span>Reconnecting...</span>
                </>
              ) : (
                <>
                  <FaSync />
                  <span>Retry Connection</span>
                </>
              )}
            </button>
          )}
        </div>
      </div>
      
      {error && (
        <div className="bg-red-100 border-l-4 border-red-500 text-red-700 p-4 mb-6 rounded">
          <div className="flex items-center">
            <div className="py-1">
              <FaExclamationTriangle className="h-6 w-6 text-red-500 mr-4" />
            </div>
            <div className="flex-1">
              <p className="font-medium">{error}</p>
              {errorDetails && isDev && (
                <p className="text-sm mt-1 text-red-600">Details: {errorDetails}</p>
              )}
              {(connectionStatus === 'error' || connectionStatus === 'closed' || connectionStatus === 'failed') && !reconnecting && (
                <button
                  onClick={handleReconnect}
                  className="mt-2 px-3 py-1 bg-red-200 text-red-700 rounded hover:bg-red-300 transition-colors text-sm flex items-center gap-2"
                >
                  <FaSync className="h-3 w-3" />
                  <span>Retry Connection</span>
                </button>
              )}
              {reconnecting && (
                <div className="mt-2 text-sm text-red-600 flex items-center">
                  <FaSpinner className="animate-spin mr-2" />
                  <span>Attempting to reconnect...</span>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
      
      {isLoading ? (
        <div className="bg-white dark:bg-dark-card rounded-lg shadow-md p-8 text-center">
          <div className="animate-pulse">
            <div className="h-8 bg-gray-200 dark:bg-gray-700 rounded w-1/4 mx-auto mb-4"></div>
            <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-1/2 mx-auto mb-2"></div>
            <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-1/3 mx-auto"></div>
          </div>
          <p className="mt-6 text-gray-600 dark:text-gray-400">
            {connectionStatus === 'connecting' 
              ? `Connecting to ${selectedServer?.name || 'server'}...` 
              : 'Loading resources...'}
          </p>
        </div>
      ) : (
        <div>
          <div className="mb-6 flex flex-col sm:flex-row justify-between items-start sm:items-center">
            <h2 className="text-xl font-semibold text-gray-800 dark:text-white mb-4 sm:mb-0">
              Resources
              {lastDataTimestamp && (
                <span className="text-sm text-gray-500 dark:text-gray-400 ml-2">
                  (Last updated: {new Date(lastDataTimestamp).toLocaleTimeString()})
                </span>
              )}
            </h2>
            <div className="w-full sm:w-auto flex flex-col sm:flex-row space-y-4 sm:space-y-0 sm:space-x-4">
              <div className="relative">
                <input
                  type="text"
                  placeholder="Search resources..."
                  className="w-full sm:w-64 px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent dark:bg-gray-800 dark:text-white"
                  value={searchTerm}
                  onChange={handleSearchChange}
                />
                <div className="absolute inset-y-0 right-0 flex items-center pr-3 pointer-events-none">
                  <svg className="h-5 w-5 text-gray-400 dark:text-gray-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                  </svg>
                </div>
              </div>
              
              <button
                className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors dark:bg-gray-800 dark:text-white"
                onClick={handleOpenFilterModal}
              >
                Add Filter
              </button>
            </div>
          </div>
          
          <ResourceGrid 
            resources={filteredAndSortedResources} 
            sortConfig={sortConfig}
            onSort={handleSort}
            onResourceClick={handleResourceClick}
            thresholds={{
              cpu: { warning: 70, critical: 90 },
              memory: { warning: 70, critical: 90 },
              disk: { warning: 70, critical: 90 }
            }}
          />
        </div>
      )}
      
      {showFilterModal && (
        <FilterModal 
          filters={[]}
          onApplyFilters={handleApplyFilters}
          onClose={handleCloseFilterModal}
        />
      )}
    </div>
  );
};

export default Dashboard; 