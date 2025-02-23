import React, { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useNodes } from '../contexts/NodeContext';
import { useWebSocketContext } from '../contexts/WebSocketContext';

const Dashboard: React.FC = () => {
  const navigate = useNavigate();
  const { state } = useNodes();
  const { socket, isConnected } = useWebSocketContext();
  const nodes = Object.values(state.nodes);

  console.log('Dashboard rendering:', { 
    nodes, 
    state, 
    socketConnected: isConnected,
    socketExists: !!socket 
  });

  if (!socket || !isConnected) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <p className="text-gray-600">
          {!socket ? 'Initializing connection...' : 'Connecting to server...'}
        </p>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <div className="container mx-auto px-4 py-8">
        <div className="flex justify-between items-center mb-6">
          <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
          <button
            onClick={() => navigate('/onboarding')}
            className="bg-blue-500 hover:bg-blue-600 text-white px-4 py-2 rounded-lg transition-colors"
          >
            Add Node
          </button>
        </div>

        {nodes.length === 0 ? (
          <div className="text-center py-12 bg-white rounded-lg shadow-sm">
            <p className="text-gray-600 text-lg">
              No nodes added yet. Click "Add Node" to get started.
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {nodes.map(node => (
              <div 
                key={node.id} 
                className="bg-white rounded-lg shadow-sm p-6 border border-gray-200"
              >
                <div className="flex justify-between items-start mb-4">
                  <div>
                    <h2 className="text-xl font-semibold text-gray-900">
                      {node.name || 'Unknown Node'}
                    </h2>
                    <p className="text-sm text-gray-600">{node.host}</p>
                  </div>
                  <span 
                    className={`px-2 py-1 rounded-full text-sm ${
                      node.status === 'online' 
                        ? 'bg-green-100 text-green-700' 
                        : 'bg-gray-100 text-gray-700'
                    }`}
                  >
                    {node.status || 'unknown'}
                  </span>
                </div>

                {node.metrics && (
                  <div className="grid grid-cols-2 gap-4 mt-4">
                    <div className="space-y-4">
                      {/* CPU Usage */}
                      <div>
                        <p className="text-sm text-gray-500">CPU Usage</p>
                        <div className="flex items-center">
                          <div className="flex-1 bg-gray-200 rounded-full h-2 mr-2">
                            <div 
                              className="bg-blue-500 h-2 rounded-full" 
                              style={{ width: `${(node.metrics.cpu * 100).toFixed(1)}%` }}
                            />
                          </div>
                          <span className="text-sm font-medium">
                            {(node.metrics.cpu * 100).toFixed(1)}%
                          </span>
                        </div>
                      </div>

                      {/* Memory Usage */}
                      <div>
                        <p className="text-sm text-gray-500">Memory Usage</p>
                        <div className="flex items-center">
                          <div className="flex-1 bg-gray-200 rounded-full h-2 mr-2">
                            <div 
                              className="bg-green-500 h-2 rounded-full" 
                              style={{ 
                                width: `${((node.metrics.memory.used / node.metrics.memory.total) * 100).toFixed(1)}%` 
                              }}
                            />
                          </div>
                          <span className="text-sm font-medium">
                            {((node.metrics.memory.used / node.metrics.memory.total) * 100).toFixed(1)}%
                          </span>
                        </div>
                        <p className="text-xs text-gray-500 mt-1">
                          {formatBytes(node.metrics.memory.used)} / {formatBytes(node.metrics.memory.total)}
                        </p>
                      </div>

                      {/* Disk Usage */}
                      <div>
                        <p className="text-sm text-gray-500">Disk Usage</p>
                        <div className="flex items-center">
                          <div className="flex-1 bg-gray-200 rounded-full h-2 mr-2">
                            <div 
                              className="bg-purple-500 h-2 rounded-full" 
                              style={{ 
                                width: `${((node.metrics.disk.used / node.metrics.disk.total) * 100).toFixed(1)}%` 
                              }}
                            />
                          </div>
                          <span className="text-sm font-medium">
                            {((node.metrics.disk.used / node.metrics.disk.total) * 100).toFixed(1)}%
                          </span>
                        </div>
                        <p className="text-xs text-gray-500 mt-1">
                          {formatBytes(node.metrics.disk.used)} / {formatBytes(node.metrics.disk.total)}
                        </p>
                      </div>
                    </div>

                    <div className="space-y-4">
                      {/* Load Average */}
                      <div>
                        <p className="text-sm text-gray-500">Load Average</p>
                        <div className="grid grid-cols-3 gap-2">
                          <div className="text-center">
                            <p className="text-sm font-medium">{node.metrics.loadavg[0].toFixed(2)}</p>
                            <p className="text-xs text-gray-500">1m</p>
                          </div>
                          <div className="text-center">
                            <p className="text-sm font-medium">{node.metrics.loadavg[1].toFixed(2)}</p>
                            <p className="text-xs text-gray-500">5m</p>
                          </div>
                          <div className="text-center">
                            <p className="text-sm font-medium">{node.metrics.loadavg[2].toFixed(2)}</p>
                            <p className="text-xs text-gray-500">15m</p>
                          </div>
                        </div>
                      </div>

                      {/* Uptime */}
                      <div>
                        <p className="text-sm text-gray-500">Uptime</p>
                        <p className="text-sm font-medium">
                          {formatUptime(node.metrics.uptime)}
                        </p>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

// Helper functions
function formatBytes(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = bytes;
  let unitIndex = 0;
  
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex++;
  }
  
  return `${value.toFixed(1)} ${units[unitIndex]}`;
}

function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  
  const parts = [];
  if (days > 0) parts.push(`${days}d`);
  if (hours > 0) parts.push(`${hours}h`);
  if (minutes > 0) parts.push(`${minutes}m`);
  
  return parts.join(' ');
}

export default Dashboard;