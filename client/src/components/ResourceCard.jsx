import React from 'react';
import { FaServer, FaDesktop, FaHdd, FaMemory, FaNetworkWired } from 'react-icons/fa';

const ResourceCard = ({ resource, onSelect }) => {
  // Determine resource type icon
  const getResourceIcon = (type) => {
    switch (type.toLowerCase()) {
      case 'vm':
      case 'qemu':
        return <FaDesktop className="resource-icon" />;
      case 'container':
      case 'lxc':
        return <FaServer className="resource-icon" />;
      case 'storage':
        return <FaHdd className="resource-icon" />;
      default:
        return <FaServer className="resource-icon" />;
    }
  };

  // Format memory usage
  const formatMemory = (memoryBytes) => {
    if (!memoryBytes && memoryBytes !== 0) return 'N/A';
    
    const gb = memoryBytes / (1024 * 1024 * 1024);
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

  return (
    <div 
      className="resource-card" 
      onClick={() => onSelect(resource)}
    >
      <div className="resource-header">
        {getResourceIcon(resource.type)}
        <h3 className="resource-name">{resource.name}</h3>
        <div 
          className="status-indicator" 
          style={{ backgroundColor: getStatusColor(resource.status) }}
        >
          {resource.status || 'Unknown'}
        </div>
      </div>
      
      <div className="resource-details">
        <div className="resource-type">{resource.type}</div>
        <div className="resource-id">ID: {resource.id}</div>
      </div>
      
      <div className="resource-metrics">
        {/* CPU Usage */}
        <div className="metric">
          <div className="metric-label">
            <FaServer /> CPU
          </div>
          <div className="progress-container">
            <div 
              className="progress-bar" 
              style={{ width: `${calculatePercentage(resource.cpu_usage, 100)}%` }}
            ></div>
          </div>
          <div className="metric-value">{resource.cpu_usage ? `${resource.cpu_usage.toFixed(1)}%` : 'N/A'}</div>
        </div>
        
        {/* Memory Usage */}
        <div className="metric">
          <div className="metric-label">
            <FaMemory /> Memory
          </div>
          <div className="progress-container">
            <div 
              className="progress-bar" 
              style={{ width: `${calculatePercentage(resource.memory_used, resource.memory_total)}%` }}
            ></div>
          </div>
          <div className="metric-value">
            {formatMemory(resource.memory_used)} / {formatMemory(resource.memory_total)}
          </div>
        </div>
        
        {/* Network Usage */}
        {resource.network_in != null && resource.network_out != null && (
          <div className="metric">
            <div className="metric-label">
              <FaNetworkWired /> Network
            </div>
            <div className="network-metrics">
              <div>↓ {(resource.network_in / (1024 * 1024)).toFixed(2)} MB/s</div>
              <div>↑ {(resource.network_out / (1024 * 1024)).toFixed(2)} MB/s</div>
            </div>
          </div>
        )}
      </div>
      
      {/* Tags */}
      {resource.tags && resource.tags.length > 0 && (
        <div className="resource-tags">
          {resource.tags.map(tag => (
            <span key={tag} className="tag">{tag}</span>
          ))}
        </div>
      )}
    </div>
  );
};

export default ResourceCard; 