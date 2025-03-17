import { useMemo } from 'react';
import { useSearchContext } from '../../../context/SearchContext';

const useActiveFilteredColumns = ({
  filters,
  guestTypeFilter,
  showStopped,
  nodeData
}) => {
  // Get search state from context
  const { searchTerm, activeSearchTerms } = useSearchContext();
  
  // Determine which columns have active filters
  const activeFilteredColumns = useMemo(() => {
    const result = {};
    
    // Resource filters
    if (filters.cpu > 0) result.cpu = true;
    if (filters.memory > 0) result.memory = true;
    if (filters.disk > 0) result.disk = true;
    if (filters.download > 0) result.netIn = true;
    if (filters.upload > 0) result.netOut = true;
    
    // Search terms - determine which column to highlight based on the search term
    // This is now only used for UI indicators in header (like filter icons)
    // not for highlighting anymore
    if (activeSearchTerms.length > 0 || searchTerm) {
      const allTerms = [...activeSearchTerms];
      if (searchTerm) allTerms.push(searchTerm);
      
      // Process node data for efficient matching
      const nodeInfo = processNodeData(nodeData);
      
      // Process each search term to determine which column(s) to highlight
      allTerms.forEach(term => {
        const termLower = term.trim().toLowerCase();
        
        // Skip empty terms
        if (!termLower) return;
        
        // Try to match the term to specific column types
        // This now returns an array of column types for partial matches
        const columnTypes = identifyColumnTypes(termLower, nodeInfo);
        
        // Apply the appropriate highlighting for all matching column types
        if (Array.isArray(columnTypes)) {
          columnTypes.forEach(columnType => {
            applyColumnHighlighting(result, columnType, termLower);
          });
        } else if (columnTypes) {
          // For backward compatibility - still handle single column type
          applyColumnHighlighting(result, columnTypes, termLower);
        }
      });
    }
    
    // Guest type filter affects the type column
    if (guestTypeFilter !== 'all') {
      result.type = true;
    }
    
    // Highlight status column when we're filtering by status (not showing all)
    if (showStopped !== null) {
      result.status = true;
    }
    
    return result;
  }, [filters, activeSearchTerms, searchTerm, guestTypeFilter, showStopped, nodeData]);

  return activeFilteredColumns;
};

// Helper function to process node data for efficient matching
function processNodeData(nodeData) {
  const nodeNames = [];
  const nodeIds = [];
  const nodePatterns = ['node', 'pve', 'prox', 'cluster', 'host', 'server'];
  
  // Add role-related terms to help with highlighting
  const rolePatterns = ['role', 'primary', 'secondary', 'pri', 'sec', 'shared'];
  
  if (nodeData && Array.isArray(nodeData)) {
    nodeData.forEach(node => {
      if (node.name) nodeNames.push(node.name.toLowerCase());
      if (node.id) nodeIds.push(node.id.toLowerCase());
      
      // Create common node name patterns
      const name = (node.name || '').toLowerCase();
      if (name.includes('-')) {
        // Add base name (e.g., 'pve' from 'pve-01')
        const baseName = name.split('-')[0];
        if (baseName && !nodePatterns.includes(baseName)) {
          nodePatterns.push(baseName);
        }
        
        // Add prefix with first digit (e.g., 'pve-0' from 'pve-01')
        const parts = name.split('-');
        if (parts.length > 1 && parts[1].length > 0) {
          const prefix = parts[0] + '-' + parts[1][0];
          if (!nodePatterns.includes(prefix)) {
            nodePatterns.push(prefix);
          }
        }
      }
    });
  }
  
  return { nodeNames, nodeIds, nodePatterns, rolePatterns };
}

// Helper function to identify which column a search term applies to
const identifyColumnTypes = (termLower, nodeInfo = {}) => {
  // Column-specific prefixes
  if (termLower.startsWith('status:')) return 'status';
  if (termLower.startsWith('type:')) return 'type';
  if (termLower.startsWith('node:')) return 'node';
  if (termLower.startsWith('role:')) return 'role';
  if (termLower.startsWith('cpu:')) return 'cpu';
  if (termLower.startsWith('memory:') || termLower.startsWith('mem:')) return 'memory';
  if (termLower.startsWith('disk:')) return 'disk';
  if (termLower.startsWith('download:') || termLower.startsWith('dl:')) return 'download';
  if (termLower.startsWith('upload:') || termLower.startsWith('ul:')) return 'upload';
  
  // Resource keywords (exact matches)
  if (['cpu', 'memory', 'mem', 'disk', 'network', 'net'].includes(termLower)) {
    if (termLower === 'network' || termLower === 'net') {
      return 'network';
    }
    if (termLower === 'mem') {
      return 'memory';
    }
    return termLower;
  }
  
  // Partial resource expressions (e.g., "cpu>")
  if (/^(cpu|mem(ory)?|disk|network|net)\s*(>|<|>=|<=|=)$/i.test(termLower)) {
    const resource = termLower.match(/^(cpu|mem(ory)?|disk|network|net)/i)[0].toLowerCase();
    if (resource === 'network' || resource === 'net') {
      return 'network';
    }
    if (resource === 'mem') {
      return 'memory';
    }
    return resource;
  }
  
  // Complete resource expressions (e.g., "cpu>50")
  const resourceExpressionRegex = /^(cpu|mem(ory)?|disk|network|net)\s*(>|<|>=|<=|=)\s*(\d+)$/i;
  const resourceMatch = termLower.match(resourceExpressionRegex);
  if (resourceMatch) {
    let resource = resourceMatch[1].toLowerCase();
    if (resource === 'network' || resource === 'net') {
      return 'network';
    }
    if (resource === 'mem') {
      return 'memory';
    }
    return resource;
  }
  
  // Direct threshold expressions without column prefix (e.g., ">50")
  const directThresholdRegex = /^(>|<|>=|<=)(\d+)$/;
  const directThresholdMatch = termLower.match(directThresholdRegex);
  if (directThresholdMatch) {
    // This is likely a CPU filter as it's the most common direct threshold
    return 'cpu';
  }
  
  // Column-specific searches with colon (e.g., "name:ubuntu")
  if (termLower.includes(':')) {
    const [prefix] = termLower.split(':', 2);
    const validPrefixes = ['name', 'id', 'node', 'status', 'type', 'cpu', 'memory', 'mem', 'disk', 'network', 'net'];
    
    if (validPrefixes.includes(prefix.trim())) {
      const prefixTrim = prefix.trim();
      if (prefixTrim === 'network' || prefixTrim === 'net') {
        return 'network';
      }
      if (prefixTrim === 'mem') {
        return 'memory';
      }
      return prefixTrim;
    }
  }
  
  // For short partial terms (1-2 characters), we'll highlight multiple potential matches
  if (termLower.length <= 2) {
    const matchingColumns = [];
    
    // Check for partial matches in all possible terms
    
    // Resource terms
    const resourceTerms = ['cpu', 'memory', 'disk', 'network', 'net'];
    for (const resource of resourceTerms) {
      if (resource.startsWith(termLower)) {
        matchingColumns.push(resource === 'network' || resource === 'net' ? 'network' : resource);
      }
    }
    
    // Type terms
    const typeTerms = ['ct', 'container', 'vm', 'virtual', 'machine', 'qemu', 'lxc'];
    if (typeTerms.some(type => type.startsWith(termLower))) {
      matchingColumns.push('type');
    }
    
    // Status terms
    const statusTerms = ['running', 'stopped', 'online', 'offline', 'active', 'inactive'];
    if (statusTerms.some(status => status.startsWith(termLower))) {
      matchingColumns.push('status');
    }
    
    // Node terms - check if term matches beginning of any node name/id
    const { nodeNames, nodeIds } = nodeInfo;
    if (nodeNames.some(name => name.startsWith(termLower)) || 
        nodeIds.some(id => id.startsWith(termLower))) {
      matchingColumns.push('node');
    }
    
    // For single letters, also include ID and name as potential matches
    if (termLower.length === 1) {
      // Always include name for single letter (could be start of any name)
      if (!matchingColumns.includes('name')) matchingColumns.push('name');
      
      // For single digit, include ID
      if (/^\d$/.test(termLower) && !matchingColumns.includes('id')) {
        matchingColumns.push('id');
      }
    }
    
    // If we found any matches, return them
    if (matchingColumns.length > 0) {
      return matchingColumns;
    }
    
    // For 1-2 character alpha terms with no specific matches, 
    // default to highlighting name column as it's the most likely target
    if (/[a-z]/i.test(termLower)) {
      return ['name'];
    }
  }
  
  // Past this point, handle longer terms (3+ chars) with more specific highlighting
  
  // Type-specific terms (exact matches)
  if (['ct', 'container', 'vm', 'virtual machine', 'qemu', 'lxc'].includes(termLower)) {
    return 'type';
  }
  
  // Status-specific terms (exact matches)
  const statusTerms = ['running', 'stopped', 'online', 'offline', 'active', 'inactive'];
  if (statusTerms.includes(termLower)) {
    return 'status';
  }
  
  // Partial status term matches (e.g., "runni" should match "running")
  // This is critical for providing good feedback while typing
  if (statusTerms.some(status => status.startsWith(termLower) || termLower.startsWith(status))) {
    return 'status';
  }
  
  // Partial type term matches (e.g., "conta" should match "container")
  const typeTerms = ['ct', 'container', 'vm', 'virtual', 'machine', 'qemu', 'lxc'];
  if (typeTerms.some(type => type.startsWith(termLower) || termLower.startsWith(type))) {
    return 'type';
  }
  
  // ID-specific terms (pure numbers)
  if (/^\d+$/.test(termLower)) {
    return 'id';
  }
  
  // Node matching
  const { nodeNames, nodeIds, nodePatterns, rolePatterns } = nodeInfo;
  
  // Exact node name/id match
  if (nodeNames.includes(termLower) || nodeIds.includes(termLower)) {
    return 'node';
  }
  
  // Node name contains term or term contains node name
  if (nodeNames.some(name => name.includes(termLower) || termLower.includes(name))) {
    return 'node';
  }
  
  // Node id contains term or term contains node id
  if (nodeIds.some(id => id.includes(termLower) || termLower.includes(id))) {
    return 'node';
  }
  
  // Term matches node patterns
  if (nodePatterns.some(pattern => termLower.includes(pattern))) {
    return 'node';
  }
  
  // Term matches role patterns
  if (rolePatterns && rolePatterns.some(pattern => termLower.includes(pattern))) {
    return 'role';
  }
  
  // Term is likely a node reference
  if (termLower.includes('-') || /^[a-z]{1,3}\d{1,2}$/i.test(termLower)) {
    return 'node';
  }
  
  // Default - this is likely a name search
  // Only highlight name if it's likely to be a name search and
  // doesn't match any partial patterns above
  if (/[a-z]/i.test(termLower) && termLower.length > 1) {
    return 'name';
  }
  
  // If we can't determine a specific column, don't highlight anything
  return null;
}

// Helper function to apply highlighting based on column type
function applyColumnHighlighting(result, columnType, term) {
  if (!columnType) return;
  
  switch (columnType) {
    case 'cpu':
      result.cpu = true;
      break;
    case 'memory':
      result.memory = true;
      break;
    case 'disk':
      result.disk = true;
      break;
    case 'network':
      result.netIn = true;
      result.netOut = true;
      break;
    case 'node':
      result.node = true;
      break;
    case 'id':
      result.id = true;
      break;
    case 'status':
      result.status = true;
      break;
    case 'type':
      result.type = true;
      break;
    case 'name':
      result.name = true;
      break;
  }
}

export default useActiveFilteredColumns; 