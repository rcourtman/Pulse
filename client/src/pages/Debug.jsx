import { useState, useEffect } from 'react';
import { FaSpinner, FaCheckCircle, FaExclamationTriangle, FaServer, FaNetworkWired, FaSearch } from 'react-icons/fa';
import { io } from 'socket.io-client';

// Get the API base URL from environment or use a default
const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:3000';
const WS_BASE_URL = API_BASE_URL;

const Debug = () => {
  const [apiStatus, setApiStatus] = useState('checking');
  const [wsStatus, setWsStatus] = useState('checking');
  const [nodes, setNodes] = useState([]);
  const [nodesStatus, setNodesStatus] = useState('checking');
  const [logs, setLogs] = useState([]);
  const [selectedNode, setSelectedNode] = useState(null);
  const [wsConnection, setWsConnection] = useState(null);
  const [wsMessages, setWsMessages] = useState([]);
  const [searchTerm, setSearchTerm] = useState('');
  const [searchResults, setSearchResults] = useState(null);

  // Add a log message
  const addLog = (message, type = 'info') => {
    const timestamp = new Date().toISOString();
    setLogs(prev => [...prev, { timestamp, message, type }]);
  };

  // Check API health
  useEffect(() => {
    const checkApiHealth = async () => {
      try {
        addLog(`Checking API health at ${API_BASE_URL}/api/health`);
        setApiStatus('checking');
        
        const response = await fetch(`${API_BASE_URL}/api/health`);
        
        if (response.ok) {
          const data = await response.json();
          addLog(`API health check successful: ${JSON.stringify(data)}`, 'success');
          setApiStatus('connected');
        } else {
          addLog(`API health check failed: ${response.status} ${response.statusText}`, 'error');
          setApiStatus('error');
        }
      } catch (error) {
        addLog(`API health check error: ${error.message}`, 'error');
        setApiStatus('error');
      }
    };
    
    checkApiHealth();
  }, []);

  // Fetch nodes
  useEffect(() => {
    const fetchNodes = async () => {
      try {
        addLog(`Fetching nodes from ${API_BASE_URL}/api/nodes`);
        setNodesStatus('checking');
        
        const response = await fetch(`${API_BASE_URL}/api/nodes`);
        
        if (response.ok) {
          const data = await response.json();
          addLog(`Fetched ${data.length} nodes`, 'success');
          setNodes(data);
          setNodesStatus('success');
          
          // Select the first node if available
          if (data.length > 0 && !selectedNode) {
            setSelectedNode(data[0]);
            addLog(`Selected node: ${data[0].name}`, 'info');
          }
        } else {
          const errorText = await response.text();
          addLog(`Failed to fetch nodes: ${response.status} ${response.statusText} - ${errorText}`, 'error');
          setNodesStatus('error');
        }
      } catch (error) {
        addLog(`Error fetching nodes: ${error.message}`, 'error');
        setNodesStatus('error');
      }
    };
    
    fetchNodes();
  }, [selectedNode]);

  // Test WebSocket connection using Socket.IO
  useEffect(() => {
    // Close existing connection
    if (wsConnection) {
      addLog('Closing existing Socket.IO connection');
      wsConnection.disconnect();
      setWsConnection(null);
    }
    
    if (!selectedNode) {
      return;
    }
    
    try {
      // Ensure we're using the correct format for the node ID
      const nodeId = selectedNode.id.toString();
      // Make sure we're using the full node ID format (node-X)
      const fullNodeId = nodeId.startsWith('node-') ? nodeId : `node-${nodeId}`;
      
      // Create Socket.IO connection to the namespace
      const socketUrl = `${WS_BASE_URL}`;
      const socketNamespace = `/api/ws`;
      
      addLog(`Testing Socket.IO connection to ${socketUrl} with namespace ${socketNamespace}`);
      setWsStatus('connecting');
      
      // Create Socket.IO client with explicit configuration
      const socket = io(`${socketUrl}${socketNamespace}`, {
        path: '/socket.io',
        transports: ['websocket', 'polling'],
        reconnection: true,
        reconnectionAttempts: 5,
        reconnectionDelay: 1000,
        reconnectionDelayMax: 5000,
        timeout: 20000,
        autoConnect: true,
        query: { nodeId: fullNodeId }
      });
      
      setWsConnection(socket);
      
      // Set a timeout to detect stalled connections
      const connectionTimeout = setTimeout(() => {
        if (socket.connected === false) {
          addLog('Socket.IO connection attempt timed out after 10 seconds', 'error');
          setWsStatus('error');
          socket.disconnect();
        }
      }, 10000);
      
      // Socket.IO event handlers
      socket.on('connect', () => {
        addLog(`Socket.IO connected with ID: ${socket.id}`, 'success');
        clearTimeout(connectionTimeout);
        
        // Update status to connected immediately when Socket.IO connects
        setWsStatus('connected');
        
        // Emit a message to join the specific node namespace
        socket.emit('join', { nodeId: fullNodeId });
        addLog(`Sent join request for node ${fullNodeId}`, 'info');
      });
      
      socket.on('connecting', (data) => {
        addLog(`Connecting to ${data.nodeName}...`, 'info');
        setWsStatus('connecting');
      });
      
      socket.on('connected', (data) => {
        addLog(`Connected to ${data.nodeName}`, 'success');
        setWsStatus('connected');
      });
      
      socket.on('data', (data) => {
        addLog(`Received data with ${data.resources ? data.resources.length : 0} resources`, 'success');
        setWsStatus('connected');
        setWsMessages(prev => [...prev, data]);
      });
      
      socket.on('error', (error) => {
        addLog(`Socket.IO error: ${error.message || 'Unknown error'}`, 'error');
        if (error.code) {
          addLog(`Error code: ${error.code}`, 'error');
        }
        setWsStatus('error');
      });
      
      socket.on('disconnect', (reason) => {
        clearTimeout(connectionTimeout);
        addLog(`Socket.IO disconnected: ${reason}`, 'warning');
        
        // Provide more specific information based on the disconnect reason
        switch (reason) {
          case 'io server disconnect':
            addLog('The server has forcefully disconnected the connection', 'error');
            break;
          case 'io client disconnect':
            addLog('The client has manually disconnected the connection', 'info');
            break;
          case 'ping timeout':
            addLog('The server did not send a PING within the expected time', 'error');
            break;
          case 'transport close':
            addLog('The connection was closed (network issue)', 'error');
            break;
          case 'transport error':
            addLog('The connection encountered an error', 'error');
            break;
          default:
            addLog(`Disconnected: ${reason}`, 'warning');
            break;
        }
        
        setWsStatus('closed');
      });
      
      socket.on('reconnect', (attemptNumber) => {
        addLog(`Socket.IO reconnected after ${attemptNumber} attempts`, 'success');
      });
      
      socket.on('reconnect_attempt', (attemptNumber) => {
        addLog(`Socket.IO reconnection attempt ${attemptNumber}`, 'info');
      });
      
      socket.on('reconnect_error', (error) => {
        addLog(`Socket.IO reconnection error: ${error.message}`, 'error');
      });
      
      socket.on('reconnect_failed', () => {
        addLog('Socket.IO failed to reconnect after all attempts', 'error');
        setWsStatus('error');
      });
      
      socket.on('connect_error', (error) => {
        addLog(`Socket.IO connection error: ${error.message}`, 'error');
        setWsStatus('error');
        
        // Add more detailed error information
        if (error.description) {
          addLog(`Error details: ${error.description}`, 'error');
        }
        
        // Automatically attempt to reconnect after a delay
        setTimeout(() => {
          if (socket && !socket.connected) {
            addLog('Attempting to reconnect...', 'info');
            socket.connect();
          }
        }, 5000);
      });
      
      // Clean up on unmount
      return () => {
        clearTimeout(connectionTimeout);
        socket.disconnect();
      };
    } catch (error) {
      addLog(`Failed to create Socket.IO connection: ${error.message}`, 'error');
      setWsStatus('error');
    }
  }, [selectedNode]);

  // Function to search for container data in the WebSocket messages
  const searchContainerData = () => {
    if (!searchTerm.trim() || wsMessages.length === 0) {
      setSearchResults(null);
      return;
    }
    
    // Get the latest WebSocket message
    const latestMessage = wsMessages[wsMessages.length - 1];
    
    // Check if the message has resources
    if (!latestMessage || !latestMessage.resources) {
      addLog('No resources data found in the latest WebSocket message', 'warning');
      setSearchResults({ query: searchTerm, results: [] });
      return;
    }
    
    // Filter resources based on search term (case insensitive)
    const filteredResources = latestMessage.resources.filter(resource => 
      resource.name && resource.name.toLowerCase().includes(searchTerm.toLowerCase())
    );
    
    // Set search results
    addLog(`Found ${filteredResources.length} containers matching "${searchTerm}"`, 
      filteredResources.length > 0 ? 'success' : 'info');
    setSearchResults({ 
      query: searchTerm, 
      results: filteredResources,
      timestamp: new Date().toISOString()
    });
  };

  // Render status indicator
  const renderStatus = (status) => {
    switch (status) {
      case 'checking':
        return <FaSpinner className="animate-spin text-yellow-500" />;
      case 'connected':
      case 'success':
        return <FaCheckCircle className="text-green-500" />;
      case 'error':
      case 'closed':
        return <FaExclamationTriangle className="text-red-500" />;
      default:
        return <FaExclamationTriangle className="text-yellow-500" />;
    }
  };

  return (
    <div className="max-w-4xl mx-auto p-4">
      <h1 className="text-2xl font-bold mb-6">Pulse Debug Console</h1>
      
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
        <div className="bg-white dark:bg-dark-card rounded-lg shadow-md p-4">
          <h2 className="text-lg font-semibold mb-2 flex items-center dark:text-white">
            <FaServer className="mr-2 text-primary-500" />
            API Connection
          </h2>
          <div className="flex items-center mb-2">
            <span className="mr-2 dark:text-white">Status:</span>
            <span className="flex items-center">
              {renderStatus(apiStatus)}
              <span className="ml-2 dark:text-white">{apiStatus}</span>
            </span>
          </div>
          <div className="text-sm text-gray-600 dark:text-gray-400">
            <div>API URL: {API_BASE_URL}</div>
          </div>
        </div>
        
        <div className="bg-white dark:bg-dark-card rounded-lg shadow-md p-4">
          <h2 className="text-lg font-semibold mb-2 flex items-center dark:text-white">
            <FaNetworkWired className="mr-2 text-primary-500" />
            WebSocket Connection
          </h2>
          <div className="flex items-center mb-2">
            <span className="mr-2 dark:text-white">Status:</span>
            <span className="flex items-center">
              {renderStatus(wsStatus)}
              <span className="ml-2 dark:text-white">{wsStatus}</span>
            </span>
          </div>
          <div className="text-sm text-gray-600 dark:text-gray-400">
            <div>WS URL: {WS_BASE_URL}</div>
            {selectedNode && (
              <div>Connected to: {selectedNode.name}</div>
            )}
          </div>
        </div>
      </div>
      
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        <div className="md:col-span-1 bg-white dark:bg-dark-card rounded-lg shadow-md p-4">
          <h2 className="text-lg font-semibold mb-2 dark:text-white">Configured Nodes</h2>
          <div className="flex items-center mb-2">
            <span className="mr-2 dark:text-white">Status:</span>
            <span className="flex items-center">
              {renderStatus(nodesStatus)}
              <span className="ml-2 dark:text-white">{nodes.length} nodes</span>
            </span>
          </div>
          
          {nodes.length > 0 ? (
            <ul className="divide-y divide-gray-200 dark:divide-gray-700">
              {nodes.map(node => (
                <li 
                  key={node.id} 
                  className={`py-2 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 ${selectedNode?.id === node.id ? 'bg-blue-50 dark:bg-blue-900' : ''}`}
                  onClick={() => {
                    setSelectedNode(node);
                    addLog(`Selected node: ${node.name}`, 'info');
                  }}
                >
                  <div className="font-medium dark:text-white">{node.name}</div>
                  <div className="text-sm text-gray-600 dark:text-gray-400">{node.url}</div>
                </li>
              ))}
            </ul>
          ) : nodesStatus === 'checking' ? (
            <div className="text-center py-4">
              <FaSpinner className="animate-spin text-primary-500 mx-auto mb-2" />
              <p>Loading nodes...</p>
            </div>
          ) : (
            <div className="text-center py-4 text-gray-500 dark:text-gray-400">
              No nodes configured
            </div>
          )}
        </div>
        
        <div className="md:col-span-2 bg-white dark:bg-dark-card rounded-lg shadow-md p-4">
          <h2 className="text-lg font-semibold mb-2 dark:text-white">Debug Logs</h2>
          <div className="h-64 overflow-y-auto bg-gray-100 dark:bg-gray-800 p-2 rounded text-sm font-mono">
            {logs.map((log, index) => (
              <div 
                key={index} 
                className={`mb-1 ${
                  log.type === 'error' ? 'text-red-600 dark:text-red-400' : 
                  log.type === 'success' ? 'text-green-600 dark:text-green-400' : 
                  log.type === 'warning' ? 'text-yellow-600 dark:text-yellow-400' : 
                  'text-gray-800 dark:text-gray-200'
                }`}
              >
                <span className="text-gray-500 dark:text-gray-400">[{log.timestamp.split('T')[1].split('.')[0]}]</span> {log.message}
              </div>
            ))}
            {logs.length === 0 && (
              <div className="text-gray-500 dark:text-gray-400 italic">No logs yet</div>
            )}
          </div>
        </div>
      </div>
      
      {wsStatus === 'error' && (
        <div className="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg p-4 mb-6">
          <h2 className="text-lg font-semibold mb-2 text-red-700 dark:text-red-400">WebSocket Connection Error</h2>
          <p className="mb-2 dark:text-gray-300">There was an error connecting to the Proxmox server. This could be due to:</p>
          <ul className="list-disc pl-5 mb-2 text-sm dark:text-gray-300">
            <li>The Proxmox server is not reachable at the configured URL</li>
            <li>Invalid API token credentials</li>
            <li>Self-signed SSL certificate issues</li>
            <li>Firewall blocking the connection</li>
          </ul>
          <p className="text-sm dark:text-gray-400">Check the logs above for more specific error details.</p>
        </div>
      )}
      
      {wsMessages.length > 0 && (
        <div className="bg-white dark:bg-dark-card rounded-lg shadow-md p-4 mb-6">
          <h2 className="text-lg font-semibold mb-2 dark:text-white">Latest WebSocket Data</h2>
          
          {/* Search container functionality */}
          <div className="mb-4 flex">
            <div className="relative flex-1">
              <input
                type="text"
                placeholder="Search for container by name..."
                className="w-full px-4 py-2 pr-10 rounded-md border border-gray-300 dark:border-gray-600 focus:outline-none focus:ring-2 focus:ring-primary-500 dark:bg-gray-800 dark:text-white"
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                onKeyPress={(e) => e.key === 'Enter' && searchContainerData()}
              />
              <FaSearch className="absolute right-3 top-3 text-gray-400 dark:text-gray-500" />
            </div>
            <button
              className="ml-2 px-4 py-2 bg-primary-500 text-white rounded-md hover:bg-primary-600 focus:outline-none focus:ring-2 focus:ring-primary-500 dark:bg-primary-600 dark:hover:bg-primary-700"
              onClick={searchContainerData}
            >
              Search
            </button>
          </div>

          {/* Search results */}
          {searchResults && (
            <div className="mb-4 bg-gray-50 dark:bg-gray-800 p-3 rounded-md">
              <h3 className="text-md font-medium mb-2 dark:text-white">
                Search Results for "{searchResults.query}" 
                <span className="text-sm font-normal text-gray-500 dark:text-gray-400 ml-2">
                  {new Date(searchResults.timestamp).toLocaleTimeString()}
                </span>
              </h3>
              
              {searchResults.results.length === 0 ? (
                <p className="text-gray-600 dark:text-gray-400">No containers found matching your search term.</p>
              ) : (
                <div className="overflow-x-auto">
                  {searchResults.results.map((resource, index) => (
                    <div key={index} className="mb-3 p-2 border border-gray-200 dark:border-gray-700 rounded-md dark:bg-gray-700">
                      <h4 className="font-medium dark:text-white">{resource.name}</h4>
                      <div className="grid grid-cols-2 gap-2 mt-1 text-sm">
                        <div className="dark:text-gray-300"><span className="font-medium">CPU:</span> {resource.cpu}%</div>
                        <div className="dark:text-gray-300"><span className="font-medium">Memory:</span> {resource.memory}%</div>
                        <div className="dark:text-gray-300"><span className="font-medium">Disk:</span> {resource.disk}%</div>
                        <div className="dark:text-gray-300"><span className="font-medium">Status:</span> {resource.status}</div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
          
          <div className="overflow-x-auto">
            <pre className="bg-gray-100 dark:bg-gray-800 p-3 rounded text-sm dark:text-gray-300">
              {JSON.stringify(wsMessages[wsMessages.length - 1], null, 2)}
            </pre>
          </div>
        </div>
      )}
      
      <div className="bg-white dark:bg-dark-card rounded-lg shadow-md p-4">
        <h2 className="text-lg font-semibold mb-2 dark:text-white">Environment Information</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <h3 className="font-medium mb-1 dark:text-white">Client Environment</h3>
            <ul className="text-sm text-gray-600 dark:text-gray-400">
              <li>API URL: {API_BASE_URL}</li>
              <li>WebSocket URL: {WS_BASE_URL}</li>
              <li>Browser: {navigator.userAgent}</li>
            </ul>
          </div>
          <div>
            <h3 className="font-medium mb-1 dark:text-white">Connection Details</h3>
            <ul className="text-sm text-gray-600 dark:text-gray-400">
              <li>API Status: {apiStatus}</li>
              <li>WebSocket Status: {wsStatus}</li>
              <li>Nodes Status: {nodesStatus}</li>
              <li>WebSocket Messages: {wsMessages.length}</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Debug; 