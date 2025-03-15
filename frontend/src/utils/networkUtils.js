import { sliderValueToNetworkRate } from './formatters';

// Helper function to extract numeric ID from strings like "node-1-ct-105"
export const extractNumericId = (fullId) => {
  if (!fullId) return '';
  
  // Try to extract the last numeric part from the ID
  const match = fullId.match(/(\d+)$/);
  if (match && match[1]) {
    return match[1];
  }
  
  // Fallback to the original ID if no numeric part is found
  return fullId;
};

// Helper function to get the node name from the node ID
export const getNodeName = (nodeId, nodeData) => {
  if (!nodeId || !nodeData || nodeData.length === 0) return nodeId;
  
  // Find the node in the nodeData array
  const node = nodeData.find(node => node.id === nodeId);
  
  // Return the node name if found, otherwise return the node ID
  return node ? node.name : nodeId;
};

// Function to get metrics for a specific guest
export const getMetricsForGuest = (guestId, metricsData) => {
  if (!metricsData || !guestId) return null;
  
  return {
    cpu: metricsData.cpu?.[guestId] || null,
    memory: metricsData.memory?.[guestId] || null,
    disk: metricsData.disk?.[guestId] || null,
    network: metricsData.network?.[guestId] || null
  };
};

// Function to filter guests based on selected node
export const getNodeFilteredGuests = (guests, selectedNode) => {
  if (!guests || !Array.isArray(guests)) {
    console.warn('getNodeFilteredGuests: guests is not an array', guests);
    return [];
  }
  
  if (selectedNode === 'all') {
    return guests;
  }
  
  console.log('Filtering guests for node:', selectedNode);
  
  // Filter guests based on the node property from the API
  const filteredGuests = guests.filter(guest => {
    // Extract the node ID from the guest's node property
    const nodeIdFromApi = guest.node;
    
    // If the node property doesn't exist, include the guest in all nodes
    if (!nodeIdFromApi) {
      console.log('Guest has no node property:', guest.id, guest.name);
      return true;
    }
    
    // Convert node IDs between formats
    // The API might return "node-1" or just "node-1"
    // The selectedNode from the dropdown is in the format "node1", "node2", etc.
    
    // First, try direct match
    if (nodeIdFromApi === selectedNode) {
      return true;
    }
    
    // Try converting "node-1" to "node1" format
    const normalizedNodeId = nodeIdFromApi.replace('-', '');
    if (normalizedNodeId === selectedNode) {
      return true;
    }
    
    // Try extracting just the number part for comparison
    const nodeNumber = nodeIdFromApi.match(/\d+$/)?.[0];
    const selectedNodeNumber = selectedNode.match(/\d+$/)?.[0];
    
    if (nodeNumber && selectedNodeNumber && nodeNumber === selectedNodeNumber) {
      return true;
    }
    
    // Try matching by prefix (e.g., "pve" in "pve-1" matches "pve1")
    const nodePrefix = nodeIdFromApi.match(/^([a-zA-Z]+)/)?.[1];
    const selectedPrefix = selectedNode.match(/^([a-zA-Z]+)/)?.[1];
    
    if (nodePrefix && selectedPrefix && nodePrefix === selectedPrefix) {
      const nodeNum = nodeIdFromApi.match(/\d+$/)?.[0];
      const selectedNum = selectedNode.match(/\d+$/)?.[0];
      
      if (nodeNum && selectedNum && nodeNum === selectedNum) {
        return true;
      }
    }
    
    return false;
  });
  
  console.log('Filtered guests:', filteredGuests.length, 'of', guests.length);
  return filteredGuests;
};

// Function to sort and filter data
export const getSortedAndFilteredData = (
  data,
  sortConfig,
  filters,
  showStopped,
  activeSearchTerms,
  searchTerm,
  metricsData,
  guestTypeFilter,
  nodeData
) => {
  if (!data || !Array.isArray(data) || data.length === 0) {
    return [];
  }
  
  // Filter by guest type if specified
  let filteredData = [...data];
  if (guestTypeFilter !== 'all') {
    const isVM = guestTypeFilter === 'vm';
    filteredData = filteredData.filter(guest => 
      isVM ? guest.type === 'qemu' : guest.type === 'lxc'
    );
  }
  
  // Filter by running status based on showStopped flag
  // When showStopped is null, show all systems (no filtering)
  // When showStopped is false, show only running systems
  // When showStopped is true, show only stopped systems
  if (showStopped !== null) {
    if (showStopped) {
      filteredData = filteredData.filter(guest => guest.status.toLowerCase() !== 'running');
    } else {
      filteredData = filteredData.filter(guest => guest.status.toLowerCase() === 'running');
    }
  }
  
  // Apply search terms
  if (activeSearchTerms.length > 0 || searchTerm) {
    const terms = [...activeSearchTerms];
    if (searchTerm && !terms.includes(searchTerm)) {
      terms.push(searchTerm);
    }
    
    if (terms.length > 0) {
      filteredData = filteredData.filter(guest => {
        // Check each search term
        return terms.every(term => {
          const termLower = term.toLowerCase().trim();
          
          // Handle OR search with spaces
          const spaceTerms = termLower.split(' ').map(t => t.trim()).filter(t => t);
          if (spaceTerms.length > 1) {
            // For OR search, at least one term must match
            return spaceTerms.some(spaceTerm => {
              return matchesTerm(guest, spaceTerm, nodeData, metricsData);
            });
          }
          
          // Handle OR search with pipe character (backward compatibility)
          if (termLower.includes('|')) {
            const orTerms = termLower.split('|').map(t => t.trim()).filter(t => t);
            // For OR search, at least one term must match
            return orTerms.some(orTerm => {
              return matchesTerm(guest, orTerm, nodeData, metricsData);
            });
          }
          
          // Regular term matching (single term)
          return matchesTerm(guest, termLower, nodeData, metricsData);
        });
      });
    }
  }
  
  // Function to check if a guest matches a single term
  function matchesTerm(guest, termLower, nodeData, metricsData) {
    // CHECK FOR PARTIAL EXPRESSIONS FIRST - Don't filter when user is typing an incomplete expression
    // This handles cases like 'cpu>' which shouldn't filter anything yet 
    if (/^(cpu|mem(ory)?|disk|network|net)\s*(>|<|>=|<=|=)$/i.test(termLower)) {
      // Return true for all items to show everything while the user is still typing
      return true;
    }
    
    // Check for resource metric expressions like cpu>50, memory<20, etc.
    const resourceExpressionRegex = /^(cpu|mem(ory)?|disk|network|net)\s*(>|<|>=|<=|=)\s*(\d+)$/i;
    const match = termLower.match(resourceExpressionRegex);
    
    if (match) {
      // Extract the resource type and handle mem/memory correctly
      let resource = match[1].toLowerCase();
      // If it's 'mem', treat it as 'memory'
      if (resource === 'mem') {
        resource = 'memory';
      }
      
      const operator = match[match.length - 2]; // The operator will be the second-to-last matched group
      const valueStr = match[match.length - 1]; // The value will be the last matched group
      const value = parseFloat(valueStr);
      
      console.log(`Resource expression match: ${resource} ${operator} ${value}`); // Debug output
      
      // Get the appropriate metric based on the resource type
      let metricValue = null;
      
      if (resource === 'cpu') {
        metricValue = metricsData?.cpu?.[guest.id]?.usage ?? 0;
      } else if (resource === 'memory') {
        metricValue = metricsData?.memory?.[guest.id]?.usagePercent ?? 0;
      } else if (resource === 'disk') {
        metricValue = metricsData?.disk?.[guest.id]?.usagePercent ?? 0;
      } else if (resource === 'network' || resource === 'net') {
        // Combine both in and out rates for 'network'
        const inRate = metricsData?.network?.[guest.id]?.inRate ?? 0;
        const outRate = metricsData?.network?.[guest.id]?.outRate ?? 0;
        metricValue = inRate + outRate;
        
        // Convert network value to Mbps for easier comparison
        metricValue = metricValue / (1024 * 1024 / 8);
      }
      
      console.log(`Metric value for ${guest.name}: ${metricValue}`); // Debug output
      
      // Compare using the specified operator
      switch (operator) {
        case '>': return metricValue > value;
        case '<': return metricValue < value;
        case '>=': return metricValue >= value;
        case '<=': return metricValue <= value;
        case '=': return metricValue === value;
        default: return false;
      }
    }
    
    // Special handling for exact type searches
    if (termLower === 'ct' || termLower === 'container') {
      return guest.type === 'lxc';
    }
    
    if (termLower === 'vm' || termLower === 'virtual machine') {
      return guest.type === 'qemu';
    }
    
    // Handle column-specific searches (using prefixes)
    if (termLower.includes(':')) {
      const [prefix, value] = termLower.split(':', 2);
      const prefixTrim = prefix.trim();
      
      // If there's no value after the colon, show all results for that column type
      // This improves the UX when the user is still typing
      if (!value || value.trim() === '') {
        const validPrefixes = ['name', 'id', 'node', 'status', 'type', 'cpu', 'memory', 'mem', 'disk', 'network', 'net'];
        
        // If it's a valid prefix, don't filter yet - return true to show all results
        // This prevents everything from disappearing while the user is typing a filter
        if (validPrefixes.includes(prefixTrim)) {
          return true;
        }
        
        // Otherwise, do a regular search
        return searchableText(guest, nodeData).includes(termLower);
      }
      
      const searchValue = value.trim();
      
      // Column-specific search logic
      switch (prefix.trim()) {
        case 'name':
          return String(guest.name || '').toLowerCase().includes(searchValue);
        case 'id':
          return String(guest.id || '').toLowerCase().includes(searchValue);
        case 'node':
          return String(guest.node || '').toLowerCase().includes(searchValue) ||
                 String(getNodeName(guest.node, nodeData) || '').toLowerCase().includes(searchValue);
        case 'status':
          return String(guest.status || '').toLowerCase().includes(searchValue);
        case 'type':
          if (searchValue === 'vm' || searchValue === 'virtual' || searchValue === 'machine') {
            return guest.type === 'qemu';
          }
          if (searchValue === 'ct' || searchValue === 'container' || searchValue === 'lxc') {
            return guest.type === 'lxc';
          }
          return String(guest.type || '').toLowerCase().includes(searchValue);
        case 'cpu':
          // Handle cpu:<value> format as cpu>=<value>
          const cpuValue = parseFloat(searchValue);
          if (!isNaN(cpuValue)) {
            const cpuMetric = metricsData?.cpu?.[guest.id]?.usage ?? 0;
            return cpuMetric >= cpuValue;
          }
          return false;
        case 'memory':
        case 'mem':
          // Handle memory:<value> and mem:<value> format as memory>=<value>
          const memoryValue = parseFloat(searchValue);
          if (!isNaN(memoryValue)) {
            const memoryMetric = metricsData?.memory?.[guest.id]?.usagePercent ?? 0;
            return memoryMetric >= memoryValue;
          }
          return false;
        case 'disk':
          // Handle disk:<value> format as disk>=<value>
          const diskValue = parseFloat(searchValue);
          if (!isNaN(diskValue)) {
            const diskMetric = metricsData?.disk?.[guest.id]?.usagePercent ?? 0;
            return diskMetric >= diskValue;
          }
          return false;
        case 'network':
        case 'net':
          // Handle network:<value> format as network>=<value>
          const networkValue = parseFloat(searchValue);
          if (!isNaN(networkValue)) {
            const inRate = metricsData?.network?.[guest.id]?.inRate ?? 0;
            const outRate = metricsData?.network?.[guest.id]?.outRate ?? 0;
            const totalRate = (inRate + outRate) / (1024 * 1024 / 8); // Convert to Mbps
            return totalRate >= networkValue;
          }
          return false;
        default:
          // If we don't recognize the prefix, treat it as a regular search
          return searchableText(guest, nodeData).includes(termLower);
      }
    }
    
    // Special handling for pure numeric IDs - must match exactly
    if (/^\d+$/.test(termLower)) {
      const guestId = extractNumericId(guest.id);
      // For single digits (1-9), treat as a prefix match for better UX while typing
      if (termLower.length <= 2) {
        return guestId.startsWith(termLower);
      }
      // For longer numbers, require exact match
      return guestId === termLower;
    }
    
    // For regular searches, check if the term is in the searchable text
    return searchableText(guest, nodeData).includes(termLower);
  }
  
  // Function to get searchable text for a guest
  function searchableText(guest, nodeData) {
    return [
      guest.name || '',
      guest.id || '',
      guest.status || '',
      guest.type || '',
      guest.node || '',
      // Include friendly node name
      getNodeName(guest.node, nodeData) || '',
      // Include type labels for better searching
      guest.type === 'qemu' ? 'vm virtual machine' : '',
      guest.type === 'lxc' ? 'ct container' : '',
      // Include status labels for better searching
      guest.status?.toLowerCase() === 'running' ? 'online active' : 'offline inactive stopped',
      // Include resource metrics for easy finding
      `cpu ${metricsData?.cpu?.[guest.id]?.usage ?? 0}`,
      `memory ${metricsData?.memory?.[guest.id]?.usagePercent ?? 0}`,
      `disk ${metricsData?.disk?.[guest.id]?.usagePercent ?? 0}`
    ].map(val => String(val).toLowerCase()).join(' ');
  }
  
  // Apply metric filters
  if (filters) {
    // CPU filter
    if (filters.cpu > 0) {
      filteredData = filteredData.filter(guest => {
        const metrics = metricsData?.cpu?.[guest.id];
        return metrics && metrics.usage >= filters.cpu;
      });
    }
    
    // Memory filter
    if (filters.memory > 0) {
      filteredData = filteredData.filter(guest => {
        const metrics = metricsData?.memory?.[guest.id];
        return metrics && metrics.usagePercent >= filters.memory;
      });
    }
    
    // Disk filter
    if (filters.disk > 0) {
      filteredData = filteredData.filter(guest => {
        const metrics = metricsData?.disk?.[guest.id];
        return metrics && metrics.usagePercent >= filters.disk;
      });
    }
    
    // Download filter
    if (filters.download > 0) {
      const bytesPerSecondThreshold = sliderValueToNetworkRate(filters.download);
      filteredData = filteredData.filter(guest => {
        const metrics = metricsData?.network?.[guest.id];
        return metrics && metrics.inRate >= bytesPerSecondThreshold;
      });
    }
    
    // Upload filter
    if (filters.upload > 0) {
      const bytesPerSecondThreshold = sliderValueToNetworkRate(filters.upload);
      filteredData = filteredData.filter(guest => {
        const metrics = metricsData?.network?.[guest.id];
        return metrics && metrics.outRate >= bytesPerSecondThreshold;
      });
    }
  }
  
  // Apply sorting
  if (sortConfig) {
    // Debug logging for sorting
    console.log('Applying sort:', sortConfig, 'to', filteredData.length, 'items');
    console.log('Has metrics data?', metricsData ? 'Yes' : 'No', 
      metricsData ? {
        cpu: Object.keys(metricsData.cpu || {}).length,
        memory: Object.keys(metricsData.memory || {}).length,
        disk: Object.keys(metricsData.disk || {}).length,
        network: Object.keys(metricsData.network || {}).length
      } : 'None');
    
    filteredData.sort((a, b) => {
      // Handle special cases for metrics-based sorting
      if (sortConfig.key === 'cpu') {
        const aMetrics = metricsData?.cpu?.[a.id];
        const bMetrics = metricsData?.cpu?.[b.id];
        const aValue = aMetrics ? aMetrics.usage : 0;
        const bValue = bMetrics ? bMetrics.usage : 0;
        return sortConfig.direction === 'asc' ? aValue - bValue : bValue - aValue;
      }
      
      if (sortConfig.key === 'memory') {
        const aMetrics = metricsData?.memory?.[a.id];
        const bMetrics = metricsData?.memory?.[b.id];
        const aValue = aMetrics ? aMetrics.usagePercent : 0;
        const bValue = bMetrics ? bMetrics.usagePercent : 0;
        return sortConfig.direction === 'asc' ? aValue - bValue : bValue - aValue;
      }
      
      if (sortConfig.key === 'disk') {
        const aMetrics = metricsData?.disk?.[a.id];
        const bMetrics = metricsData?.disk?.[b.id];
        const aValue = aMetrics ? aMetrics.usagePercent : 0;
        const bValue = bMetrics ? bMetrics.usagePercent : 0;
        return sortConfig.direction === 'asc' ? aValue - bValue : bValue - aValue;
      }
      
      if (sortConfig.key === 'download') {
        const aMetrics = metricsData?.network?.[a.id];
        const bMetrics = metricsData?.network?.[b.id];
        const aValue = aMetrics ? aMetrics.inRate : 0;
        const bValue = bMetrics ? bMetrics.inRate : 0;
        return sortConfig.direction === 'asc' ? aValue - bValue : bValue - aValue;
      }
      
      if (sortConfig.key === 'upload') {
        const aMetrics = metricsData?.network?.[a.id];
        const bMetrics = metricsData?.network?.[b.id];
        const aValue = aMetrics ? aMetrics.outRate : 0;
        const bValue = bMetrics ? bMetrics.outRate : 0;
        return sortConfig.direction === 'asc' ? aValue - bValue : bValue - aValue;
      }
      
      if (sortConfig.key === 'uptime') {
        const aValue = a.uptime || 0;
        const bValue = b.uptime || 0;
        return sortConfig.direction === 'asc' ? aValue - bValue : bValue - aValue;
      }
      
      // For string-based properties
      if (sortConfig.key === 'id') {
        // Extract numeric part for better sorting
        const aId = extractNumericId(a.id);
        const bId = extractNumericId(b.id);
        
        // Try to convert to numbers for numeric sorting
        const aNum = parseInt(aId, 10);
        const bNum = parseInt(bId, 10);
        
        if (!isNaN(aNum) && !isNaN(bNum)) {
          return sortConfig.direction === 'asc' ? aNum - bNum : bNum - aNum;
        }
        
        // Fallback to string comparison
        return sortConfig.direction === 'asc' 
          ? aId.localeCompare(bId) 
          : bId.localeCompare(aId);
      }
      
      // Special case for status column (running status should be first)
      if (sortConfig.key === 'status') {
        // Normalize status values for consistent sorting
        const statusPriority = {
          'running': 1,
          'paused': 2,
          'suspended': 3,
          'stopped': 4
        };
        
        const aStatus = (a.status || '').toLowerCase();
        const bStatus = (b.status || '').toLowerCase();
        
        // Get priority or default to highest number (end of sort)
        const aPriority = statusPriority[aStatus] || 999;
        const bPriority = statusPriority[bStatus] || 999;
        
        // Sort by priority number
        if (sortConfig.direction === 'asc') {
          return aPriority - bPriority;
        } else {
          return bPriority - aPriority;
        }
      }
      
      // Special case for type column (normalize qemu/lxc to vm/ct)
      if (sortConfig.key === 'type') {
        // Normalize type values to boolean (true for VM, false for CT)
        const getTypeValue = (type) => {
          const typeStr = (type || '').toLowerCase();
          return typeStr === 'qemu';
        };
        
        const aIsVM = getTypeValue(a.type);
        const bIsVM = getTypeValue(b.type);
        
        // Simple boolean comparison
        return sortConfig.direction === 'asc'
          ? (aIsVM === bIsVM ? 0 : aIsVM ? 1 : -1)
          : (aIsVM === bIsVM ? 0 : aIsVM ? -1 : 1);
      }
      
      // Default string comparison for other fields
      const aValue = a[sortConfig.key] || '';
      const bValue = b[sortConfig.key] || '';
      
      return sortConfig.direction === 'asc' 
        ? String(aValue).localeCompare(String(bValue)) 
        : String(bValue).localeCompare(String(aValue));
    });
  }
  
  return filteredData;
};

/**
 * Calculate dynamic column widths based on visible columns
 * @param {Object} columnVisibility - Object containing column visibility state
 * @returns {Object} - Object with column IDs as keys and pixel values as values
 */
export const calculateDynamicColumnWidths = (columnVisibility) => {
  const defaultWidths = {
    node: 140,     // Moderate - node names
    type: 30,      // Very small - just "VM" or "CT"
    id: 40,        // Very small - just numeric IDs
    status: 30,    // Minimal - just an icon
    name: 250,     // Large - typically longer text
    cpu: 120,      // Medium - progress bar with percentage
    memory: 120,   // Larger - progress bar with byte values
    disk: 120,     // Larger - progress bar with byte values
    download: 80,  // Medium - network rates
    upload: 80,    // Medium - network rates
    uptime: 70     // Medium - time display
  };
  
  // Get visible columns
  const visibleColumns = Object.keys(columnVisibility).filter(key => columnVisibility[key].visible);
  
  // If no columns are visible, return default widths
  if (visibleColumns.length === 0) {
    // Return default pixel values instead of percentages
    return { ...defaultWidths };
  }
  
  // Return the raw pixel values for each column - no percentage conversion
  const adjustedWidths = {};
  visibleColumns.forEach(key => {
    adjustedWidths[key] = defaultWidths[key];
  });
  
  return adjustedWidths;
}; 