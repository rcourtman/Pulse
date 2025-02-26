import React from 'react';
import { FaServer, FaDesktop, FaHdd, FaMemory, FaNetworkWired, FaTimes } from 'react-icons/fa';

const ResourceDetails = ({ resource, onClose }) => {
  if (!resource) return null;

  // Format memory usage
  const formatMemory = (memoryBytes) => {
    if (!memoryBytes && memoryBytes !== 0) return 'N/A';
    
    const gb = memoryBytes / (1024 * 1024 * 1024);
    return `${gb.toFixed(2)} GB`;
  };

  // Format disk usage
  const formatDisk = (diskBytes) => {
    if (!diskBytes && diskBytes !== 0) return 'N/A';
    
    const gb = diskBytes / (1024 * 1024 * 1024);
    return `${gb.toFixed(2)} GB`;
  };

  // Calculate percentage for progress bars
  const calculatePercentage = (used, total) => {
    if (!used || !total) return 0;
    const percentage = (used / total) * 100;
    return Math.min(percentage, 100); // Cap at 100%
  };

  // Determine status color
  const getStatusColor = (status) => {
    if (!status) return 'gray';
    
    switch (status.toLowerCase()) {
      case 'running':
        return 'green';
      case 'stopped':
        return 'red';
      case 'paused':
        return 'yellow';
      default:
        return 'gray';
    }
  };

  // Get icon based on resource type
  const getResourceIcon = (type) => {
    switch (type?.toLowerCase()) {
      case 'vm':
      case 'qemu':
        return <FaDesktop className="resource-icon large" />;
      case 'container':
      case 'lxc':
        return <FaServer className="resource-icon large" />;
      case 'storage':
        return <FaHdd className="resource-icon large" />;
      default:
        return <FaServer className="resource-icon large" />;
    }
  };

  return (
    <div className="resource-details-modal">
      <div className="resource-details-content">
        <button className="close-button" onClick={onClose}>
          <FaTimes />
        </button>
        
        <div className="resource-details-header">
          {getResourceIcon(resource.type)}
          <div>
            <h2>{resource.name}</h2>
            <div className="resource-details-subheader">
              <span className="resource-type">{resource.type}</span>
              <span 
                className="status-indicator" 
                style={{ backgroundColor: getStatusColor(resource.status) }}
              >
                {resource.status || 'Unknown'}
              </span>
            </div>
          </div>
        </div>
        
        <div className="resource-details-body">
          <div className="resource-details-section">
            <h3>General Information</h3>
            <div className="details-grid">
              <div className="detail-item">
                <span className="detail-label">ID:</span>
                <span className="detail-value">{resource.id}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">Node:</span>
                <span className="detail-value">{resource.node || 'N/A'}</span>
              </div>
              <div className="detail-item">
                <span className="detail-label">Uptime:</span>
                <span className="detail-value">{resource.uptime ? formatUptime(resource.uptime) : 'N/A'}</span>
              </div>
              {resource.description && (
                <div className="detail-item full-width">
                  <span className="detail-label">Description:</span>
                  <span className="detail-value">{resource.description}</span>
                </div>
              )}
            </div>
          </div>
          
          <div className="resource-details-section">
            <h3>Resource Usage</h3>
            
            {/* CPU Usage */}
            <div className="detail-metric">
              <div className="metric-header">
                <FaServer /> <span>CPU Usage</span>
              </div>
              <div className="progress-container large">
                <div 
                  className="progress-bar" 
                  style={{ width: `${calculatePercentage(resource.cpu_usage, 100)}%` }}
                ></div>
              </div>
              <div className="metric-value">{resource.cpu_usage ? `${resource.cpu_usage.toFixed(1)}%` : 'N/A'}</div>
              {resource.cpu_cores && (
                <div className="metric-subtext">Cores: {resource.cpu_cores}</div>
              )}
            </div>
            
            {/* Memory Usage */}
            <div className="detail-metric">
              <div className="metric-header">
                <FaMemory /> <span>Memory Usage</span>
              </div>
              <div className="progress-container large">
                <div 
                  className="progress-bar" 
                  style={{ width: `${calculatePercentage(resource.memory_used, resource.memory_total)}%` }}
                ></div>
              </div>
              <div className="metric-value">
                {formatMemory(resource.memory_used)} / {formatMemory(resource.memory_total)}
              </div>
            </div>
            
            {/* Disk Usage */}
            {resource.disk_used != null && resource.disk_total != null && (
              <div className="detail-metric">
                <div className="metric-header">
                  <FaHdd /> <span>Disk Usage</span>
                </div>
                <div className="progress-container large">
                  <div 
                    className="progress-bar" 
                    style={{ width: `${calculatePercentage(resource.disk_used, resource.disk_total)}%` }}
                  ></div>
                </div>
                <div className="metric-value">
                  {formatDisk(resource.disk_used)} / {formatDisk(resource.disk_total)}
                </div>
              </div>
            )}
            
            {/* Network Usage */}
            {resource.network_in != null && resource.network_out != null && (
              <div className="detail-metric">
                <div className="metric-header">
                  <FaNetworkWired /> <span>Network Usage</span>
                </div>
                <div className="network-details">
                  <div className="network-detail">
                    <span>Incoming:</span>
                    <span>{(resource.network_in / (1024 * 1024)).toFixed(2)} MB/s</span>
                  </div>
                  <div className="network-detail">
                    <span>Outgoing:</span>
                    <span>{(resource.network_out / (1024 * 1024)).toFixed(2)} MB/s</span>
                  </div>
                </div>
              </div>
            )}
          </div>
          
          {/* Tags */}
          {resource.tags && resource.tags.length > 0 && (
            <div className="resource-details-section">
              <h3>Tags</h3>
              <div className="tags-container">
                {resource.tags.map(tag => (
                  <span key={tag} className="tag">{tag}</span>
                ))}
              </div>
            </div>
          )}
          
          {/* Additional Properties */}
          <div className="resource-details-section">
            <h3>Additional Properties</h3>
            <div className="details-grid">
              {Object.entries(resource)
                .filter(([key]) => !['id', 'name', 'type', 'status', 'node', 'description', 
                                    'cpu_usage', 'cpu_cores', 'memory_used', 'memory_total', 
                                    'disk_used', 'disk_total', 'network_in', 'network_out', 
                                    'uptime', 'tags'].includes(key))
                .map(([key, value]) => (
                  <div key={key} className="detail-item">
                    <span className="detail-label">{formatKey(key)}:</span>
                    <span className="detail-value">{formatValue(value)}</span>
                  </div>
                ))
              }
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

// Helper function to format uptime
const formatUptime = (seconds) => {
  if (!seconds && seconds !== 0) return 'N/A';
  
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  
  let result = '';
  if (days > 0) result += `${days}d `;
  if (hours > 0 || days > 0) result += `${hours}h `;
  result += `${minutes}m`;
  
  return result;
};

// Helper function to format property keys
const formatKey = (key) => {
  return key
    .split('_')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
};

// Helper function to format property values
const formatValue = (value) => {
  if (value === null || value === undefined) return 'N/A';
  if (typeof value === 'boolean') return value ? 'Yes' : 'No';
  if (typeof value === 'object') return JSON.stringify(value);
  return value.toString();
};

export default ResourceDetails; 