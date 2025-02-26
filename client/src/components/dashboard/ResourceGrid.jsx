import React from 'react';
import PropTypes from 'prop-types';
import { FaSort, FaSortDown, FaDesktop, FaServer, FaBox, FaSortUp } from 'react-icons/fa';
import ProgressBar from './ProgressBar';
import { ArrowDownIcon, ArrowUpIcon } from '@heroicons/react/24/outline';

const ResourceGrid = ({ 
  resources, 
  sortConfig = { key: null, direction: 'desc' }, 
  onSort = () => {},
  onResourceClick = () => {},
  thresholds = {
    cpu: { warning: 70, critical: 90 },
    memory: { warning: 70, critical: 90 },
    disk: { warning: 70, critical: 90 }
  }
}) => {
  // Get the appropriate sort icon
  const getSortIcon = (key) => {
    if (sortConfig.key !== key) {
      return <FaSort className="ml-1 text-gray-400 dark:text-gray-500" />;
    }
    
    // Show different icons based on sort direction
    return sortConfig.direction === 'asc' 
      ? <FaSortUp className="ml-1 text-primary-500" />
      : <FaSortDown className="ml-1 text-primary-500" />;
  };
  
  // Get the appropriate sort class for the column header
  const getSortClass = (key) => {
    if (sortConfig.key !== key) {
      return '';
    }
    
    return 'bg-blue-50 dark:bg-blue-900 text-blue-700 dark:text-blue-300';
  };
  
  // Handle column header click for sorting
  const handleSortClick = (key) => {
    if (typeof onSort === 'function') {
      onSort(key);
    }
  };
  
  // Get the appropriate icon for resource type
  const getResourceTypeIcon = (type) => {
    switch (type) {
      case 'vm':
        return <FaBox className="text-purple-500" />;
      case 'container':
      case 'lxc':
        return <FaServer className="text-blue-500" />;
      default:
        return <FaDesktop className="text-gray-500" />;
    }
  };
  
  // Format network speed in a human-readable format
  const formatNetworkSpeed = (bytesPerSec) => {
    if (bytesPerSec === undefined || bytesPerSec === null) return '0 B/s';
    
    // Convert bytes per second to appropriate units without any normalization factor
    if (bytesPerSec < 1024) return `${Math.round(bytesPerSec)} B/s`;
    if (bytesPerSec < 1024 * 1024) return `${Math.round(bytesPerSec / 1024)} KB/s`;
    if (bytesPerSec < 1024 * 1024 * 1024) return `${Math.round(bytesPerSec / (1024 * 1024))} MB/s`;
    return `${Math.round(bytesPerSec / (1024 * 1024 * 1024))} GB/s`;
  };
  
  // Format CPU, memory, and disk values as integers with % sign
  const formatPercentage = (value) => {
    if (value === undefined || value === null) return '0%';
    return `${Math.round(value)}%`;
  };
  
  // Format raw Proxmox CPU value (0-1 range)
  const formatProxmoxCpu = (cpu) => {
    if (cpu === undefined || cpu === null) return '0%';
    // Proxmox returns CPU as a decimal between 0-1, convert to percentage
    return `${Math.round(cpu * 100)}%`;
  };
  
  // Format raw Proxmox memory values
  const formatProxmoxMemory = (mem, maxmem) => {
    if (mem === undefined || mem === null || maxmem === undefined || maxmem === null) return '0%';
    // Calculate percentage of memory used
    return `${Math.round((mem / maxmem) * 100)}%`;
  };
  
  // Format raw Proxmox disk values
  const formatProxmoxDisk = (disk, maxdisk) => {
    if (disk === undefined || disk === null || maxdisk === undefined || maxdisk === null || maxdisk <= 0) {
      return '0%';
    }
    // Calculate percentage of disk used and cap at 100%
    const percentage = Math.min(Math.round((disk / maxdisk) * 100), 100);
    return `${percentage}%`;
  };
  
  // Check if the resource is in raw Proxmox format
  const isRawProxmoxFormat = (resource) => {
    // Raw Proxmox resources have properties like maxmem, maxdisk, cpu (as decimal)
    return resource && (resource.maxmem !== undefined || resource.maxdisk !== undefined);
  };
  
  // Get CPU value for display with core information
  const getCpuDisplay = (resource) => {
    const cpuCores = resource.cpuCores || 1;
    
    // Check if this resource reports direct percentage values
    if (resource.isDirectPercentage) {
      // For resources that report direct percentages, just display the value as is
      return `${Math.round(resource.cpu * 10) / 10}%`; // Round to 1 decimal place
    }
    
    // Use cpuPercentage if available (already normalized in Dashboard component)
    if (typeof resource.cpuPercentage === 'number') {
      // cpuPercentage is in 0-1 range, convert to percentage
      const normalizedCpu = Math.min(Math.round(resource.cpuPercentage * 100), 100);
      return `${normalizedCpu}%`;
    }
    
    // Fallback to legacy handling
    // For normal resources and Proxmox
    let cpuValue = isRawProxmoxFormat(resource) ? (resource.cpu * 100) : resource.cpu;
    
    // Normalize CPU as a percentage of total assigned cores
    if (cpuCores > 1) {
      const normalizedCpu = Math.min(Math.round((cpuValue / cpuCores)), 100);
      return `${normalizedCpu}%`;
    }
    
    // For single-core, cap at 100%
    return `${Math.min(Math.round(cpuValue), 100)}%`;
  };
  
  // Get memory value for display
  const getMemoryDisplay = (resource) => {
    // For resources that report direct percentages
    if (resource.isDirectPercentage) {
      return `${Math.round(resource.memory * 10) / 10}%`; // Round to 1 decimal place
    }
    
    if (isRawProxmoxFormat(resource)) {
      return formatProxmoxMemory(resource.mem, resource.maxmem);
    } else {
      return formatPercentage(resource.memory);
    }
  };
  
  // Get disk value for display
  const getDiskDisplay = (resource) => {
    // For resources that report direct percentages
    if (resource.isDirectPercentage) {
      return `${Math.round(resource.disk * 10) / 10}%`; // Round to 1 decimal place
    }
    
    if (isRawProxmoxFormat(resource)) {
      return formatProxmoxDisk(resource.disk, resource.maxdisk);
    } else {
      // Cap at 100% for sanity
      const diskValue = Math.min(resource.disk || 0, 100);
      return formatPercentage(diskValue);
    }
  };
  
  // Get network values for display
  const getNetworkDisplay = (resource, direction) => {
    // Check for direct networkIn/networkOut properties first (processed format)
    if (direction === 'in' && typeof resource.networkIn !== 'undefined') {
      return formatNetworkSpeed(resource.networkIn);
    } else if (direction === 'out' && typeof resource.networkOut !== 'undefined') {
      return formatNetworkSpeed(resource.networkOut);
    }
    
    // Check for raw Proxmox format (netin/netout)
    if (direction === 'in' && typeof resource.netin !== 'undefined') {
      return formatNetworkSpeed(resource.netin);
    } else if (direction === 'out' && typeof resource.netout !== 'undefined') {
      return formatNetworkSpeed(resource.netout);
    }
    
    // Check for network object format
    if (resource.network && typeof resource.network[direction] !== 'undefined') {
      return formatNetworkSpeed(resource.network[direction]);
    }
    
    // Default fallback
    return '0 B/s';
  };
  
  // Generate a stable key for a resource
  const getResourceKey = (resource) => {
    return resource.id || 
           resource._updateId || 
           resource._updateTimestamp ||
           `${resource.name}-${resource.type}-${resource.status}`;
  };
  
  // Get normalized CPU value for progress bar (as percentage of total available cores)
  const getNormalizedCpuValue = (resource) => {
    const cpuCores = resource.cpuCores || 1;
    
    // Check if this resource reports direct percentage values
    if (resource.isDirectPercentage) {
      // For resources that report direct percentages, use the value directly for the progress bar
      return Math.min(Math.round(resource.cpu), 100);
    }
    
    // Use cpuPercentage if available (already normalized in Dashboard component)
    if (typeof resource.cpuPercentage === 'number') {
      // cpuPercentage is in 0-1 range, convert to percentage
      return Math.min(Math.round(resource.cpuPercentage * 100), 100);
    }
    
    // Fallback to legacy handling
    // For normal resources and Proxmox
    const cpuValue = isRawProxmoxFormat(resource) ? (resource.cpu * 100) : resource.cpu;
    
    // For multi-core systems, divide by number of cores to get percentage of total capacity
    if (cpuCores > 1) {
      return Math.min(Math.round((cpuValue / cpuCores)), 100);
    }
    
    // For single-core systems, cap at 100%
    return Math.min(Math.round(cpuValue), 100);
  };
  
  // Get normalized memory value for progress bar
  const getNormalizedMemoryValue = (resource) => {
    // For resources reporting direct percentages
    if (resource.isDirectPercentage) {
      return Math.min(Math.round(resource.memory), 100);
    }
    
    // For Proxmox format
    if (isRawProxmoxFormat(resource)) {
      return Math.min(Math.round((resource.mem / resource.maxmem) * 100), 100);
    }
    
    // For standard format
    return Math.min(Math.round(resource.memory), 100);
  };
  
  // Get normalized disk value for progress bar
  const getNormalizedDiskValue = (resource) => {
    // For resources reporting direct percentages
    if (resource.isDirectPercentage) {
      return Math.min(Math.round(resource.disk), 100);
    }
    
    // For Proxmox format
    if (isRawProxmoxFormat(resource)) {
      return Math.min(Math.round((resource._rawDisk || resource.disk) / resource.maxdisk * 100), 100);
    }
    
    // For standard format
    return Math.min(Math.round(resource.disk), 100);
  };
  
  // Get status dot color based on resource status
  const getStatusDotColor = (status) => {
    switch (status) {
      case 'running':
        return 'bg-green-500';
      case 'stopped':
        return 'bg-red-500';
      default:
        return 'bg-yellow-500';
    }
  };
  
  // Name header with dropdown for type and status sorting
  const renderNameHeader = () => {
    return (
      <div className="relative group">
        <div className="flex items-center cursor-pointer" onClick={() => handleSortClick('name')}>
          Name
          {getSortIcon('name')}
        </div>
        <div className="absolute hidden group-hover:block top-full left-0 mt-1 bg-white dark:bg-dark-card shadow-lg rounded-md z-10">
          <div className="p-2">
            <div 
              className="px-4 py-2 text-xs hover:bg-gray-100 dark:hover:bg-gray-700 cursor-pointer flex items-center"
              onClick={(e) => { e.stopPropagation(); handleSortClick('type'); }}
            >
              Sort by Type {sortConfig.key === 'type' && (sortConfig.direction === 'asc' ? <FaSortUp className="ml-1" /> : <FaSortDown className="ml-1" />)}
            </div>
            <div 
              className="px-4 py-2 text-xs hover:bg-gray-100 dark:hover:bg-gray-700 cursor-pointer flex items-center"
              onClick={(e) => { e.stopPropagation(); handleSortClick('status'); }}
            >
              Sort by Status {sortConfig.key === 'status' && (sortConfig.direction === 'asc' ? <FaSortUp className="ml-1" /> : <FaSortDown className="ml-1" />)}
            </div>
          </div>
        </div>
      </div>
    );
  };
  
  return (
    <div className="w-full">
      {resources.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-8 bg-white dark:bg-dark-card rounded-lg shadow">
          <div className="text-lg text-gray-500 dark:text-gray-400">No resources found</div>
        </div>
      ) : (
        <div className="shadow overflow-hidden border-b border-gray-200 dark:border-gray-700 rounded-lg">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th scope="col" className={`px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-44 ${getSortClass('name')}`}>
                  {renderNameHeader()}
                </th>
                <th scope="col" className={`px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-28 ${getSortClass('cpu')}`}>
                  <div className="flex items-center cursor-pointer" onClick={() => handleSortClick('cpu')}>
                    CPU
                    {getSortIcon('cpu')}
                  </div>
                </th>
                <th scope="col" className={`px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-28 ${getSortClass('memory')}`}>
                  <div className="flex items-center cursor-pointer" onClick={() => handleSortClick('memory')}>
                    Memory
                    {getSortIcon('memory')}
                  </div>
                </th>
                <th scope="col" className={`px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-28 ${getSortClass('disk')}`}>
                  <div className="flex items-center cursor-pointer" onClick={() => handleSortClick('disk')}>
                    Disk
                    {getSortIcon('disk')}
                  </div>
                </th>
                <th scope="col" className={`px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-20 ${getSortClass('networkIn')}`}>
                  <div className="flex items-center cursor-pointer" onClick={() => handleSortClick('networkIn')}>
                    <ArrowDownIcon className="h-4 w-4 text-green-500 mr-1" />
                    Net In
                    {getSortIcon('networkIn')}
                  </div>
                </th>
                <th scope="col" className={`px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-20 ${getSortClass('networkOut')}`}>
                  <div className="flex items-center cursor-pointer" onClick={() => handleSortClick('networkOut')}>
                    <ArrowUpIcon className="h-4 w-4 text-blue-500 mr-1" />
                    Net Out
                    {getSortIcon('networkOut')}
                  </div>
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-dark-card divide-y divide-gray-200 dark:divide-gray-700">
              {resources.map((resource) => (
                <tr 
                  key={getResourceKey(resource)}
                  onClick={() => onResourceClick(resource)}
                  className="hover:bg-gray-50 dark:hover:bg-gray-800 cursor-pointer"
                  data-update-timestamp={resource._updateTimestamp}
                >
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100 w-44 truncate">
                    <div className="flex items-center">
                      <div 
                        className={`h-3 w-3 flex-shrink-0 rounded-full mr-2 ${getStatusDotColor(resource.status)}`} 
                        title={`Status: ${resource.status.charAt(0).toUpperCase() + resource.status.slice(1)}`}
                      ></div>
                      <div className="flex items-center truncate">
                        <span className="flex-shrink-0">{getResourceTypeIcon(resource.type)}</span>
                        <span className="ml-2 truncate" title={resource.name}>{resource.name}</span>
                      </div>
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300 w-28">
                    <ProgressBar 
                      value={getNormalizedCpuValue(resource)} 
                      label={getCpuDisplay(resource)}
                      thresholds={thresholds.cpu}
                    />
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300 w-28">
                    <ProgressBar 
                      value={getNormalizedMemoryValue(resource)} 
                      label={getMemoryDisplay(resource)}
                      thresholds={thresholds.memory}
                    />
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300 w-28">
                    <ProgressBar 
                      value={getNormalizedDiskValue(resource)} 
                      label={getDiskDisplay(resource)}
                      thresholds={thresholds.disk}
                    />
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300 w-20">
                    <div className="flex items-center truncate">
                      <ArrowDownIcon className="h-4 w-4 flex-shrink-0 text-green-500 mr-1" />
                      <span className="truncate">{getNetworkDisplay(resource, 'in')}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300 w-20">
                    <div className="flex items-center truncate">
                      <ArrowUpIcon className="h-4 w-4 flex-shrink-0 text-blue-500 mr-1" />
                      <span className="truncate">{getNetworkDisplay(resource, 'out')}</span>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

ResourceGrid.propTypes = {
  resources: PropTypes.array.isRequired,
  sortConfig: PropTypes.object,
  onSort: PropTypes.func,
  onResourceClick: PropTypes.func,
  thresholds: PropTypes.object
};

export default ResourceGrid; 