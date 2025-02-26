import React, { useState } from 'react';
import { FaServer, FaChevronDown, FaCircle } from 'react-icons/fa';

const ServerSelector = ({ 
  servers = [], 
  selectedServerId, 
  selectedServer,
  onServerChange,
  connectionStatus
}) => {
  // Add state to track dropdown open/closed
  const [isOpen, setIsOpen] = useState(false);
  
  // Use either selectedServer.id or selectedServerId
  const currentServerId = selectedServer?.id || selectedServerId;
  
  // Find the selected server object
  const currentServer = selectedServer || servers.find(server => server.id === currentServerId);
  
  // Get connection status indicator color
  const getStatusColor = () => {
    if (!connectionStatus) return 'text-gray-400 dark:text-gray-500';
    
    switch (connectionStatus) {
      case 'connected':
        return 'text-green-500';
      case 'connecting':
        return 'text-yellow-500';
      case 'error':
      case 'closed':
        return 'text-red-500';
      default:
        return 'text-gray-400 dark:text-gray-500';
    }
  };
  
  // Handle server selection and close dropdown
  const handleServerSelect = (serverId) => {
    onServerChange(serverId);
    setIsOpen(false);
  };
  
  return (
    <div className="relative inline-block text-left w-full sm:w-auto">
      <div>
        <button
          type="button"
          className="inline-flex justify-between items-center w-full rounded-md border border-gray-300 dark:border-gray-600 shadow-sm px-4 py-2 bg-white dark:bg-dark-card text-sm font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary-500 dark:focus:ring-offset-dark-background"
          id="server-selector"
          aria-expanded={isOpen}
          aria-haspopup="true"
          onClick={() => setIsOpen(!isOpen)}
        >
          <div className="flex items-center">
            <FaServer className="mr-2 text-gray-500 dark:text-gray-400" />
            <span>{currentServer?.name || 'Select Server'}</span>
            <FaCircle className={`ml-2 text-xs ${getStatusColor()}`} />
          </div>
          <FaChevronDown className="-mr-1 ml-2 h-5 w-5" aria-hidden="true" />
        </button>
      </div>

      {isOpen && (
        <div
          className="origin-top-right absolute right-0 mt-2 w-56 rounded-md shadow-lg bg-white dark:bg-dark-card ring-1 ring-black ring-opacity-5 dark:ring-gray-700 focus:outline-none z-10"
          role="menu"
          aria-orientation="vertical"
          aria-labelledby="server-selector"
          tabIndex="-1"
        >
          <div className="py-1" role="none">
            {servers.map((server) => (
              <button
                key={server.id}
                onClick={() => handleServerSelect(server.id)}
                className={`
                  block w-full text-left px-4 py-2 text-sm
                  ${server.id === currentServerId ? 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white' : 'text-gray-700 dark:text-gray-200'}
                  hover:bg-gray-100 dark:hover:bg-gray-700 hover:text-gray-900 dark:hover:text-white
                `}
                role="menuitem"
                tabIndex="-1"
              >
                <div className="flex items-center">
                  <FaServer className="mr-2 text-gray-500 dark:text-gray-400" />
                  {server.name}
                </div>
              </button>
            ))}
            
            {servers.length === 0 && (
              <div className="px-4 py-2 text-sm text-gray-500 dark:text-gray-400">
                No servers available
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default ServerSelector; 