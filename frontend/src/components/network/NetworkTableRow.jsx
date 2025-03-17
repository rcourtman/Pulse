import React, { useMemo } from 'react';
import {
  TableRow,
  TableCell,
  Typography,
  Box,
  Chip,
  Tooltip,
  alpha,
  useTheme
} from '@mui/material';
import { StatusIndicator, ProgressWithLabel } from './UIComponents';
import { formatBytes, formatNetworkRate, formatUptime, formatPercentage, formatBytesWithUnit } from '../../utils/formatters';
import { calculateDynamicColumnWidths } from '../../utils/networkUtils';
import { useSearchContext } from '../../context/SearchContext';
import { useUserSettings } from '../../context/UserSettingsContext';
import { getNodeColor, getNodeTextColor } from '../../utils/colorUtils';

const NetworkTableRow = ({ 
  guest, 
  metrics, 
  columnVisibility, 
  getNodeName,
  extractNumericId,
  columnOrder,
  activeFilteredColumns = {},
  thresholdColumn,
  thresholdValue,
  sharedGuestIds = {},
  filters
}) => {
  const theme = useTheme();
  
  // DEBUG: Log the guest data to see what hastate values are coming in
  // More detailed logging for the first few guests to keep logs manageable
  const guestIdNum = parseInt(guest?.id?.toString() || '0', 10);
  if (guestIdNum <= 5) {
    console.log('Guest details:', { 
      id: guest.id, 
      name: guest.name,
      node: guest.node,
      status: guest.status,
      hastate: guest.hastate, 
      shared: guest.shared,
      isRunning: guest.status?.toLowerCase() === 'running'
    });
  }
  
  // Get search terms for highlighting
  const { searchTerm, activeSearchTerms } = useSearchContext();
  
  // Get user settings for compact mode
  const { getTableCellPadding } = useUserSettings();
  
  // Get the node name for this guest
  const nodeName = getNodeName(guest?.node);
  
  // Generate a text color for the node name
  const nodeTextColor = useMemo(() => {
    return getNodeTextColor(nodeName, theme.palette.mode);
  }, [nodeName, theme.palette.mode]);
  
  // Combine all active search terms for highlighting
  const allSearchTerms = useMemo(() => {
    const terms = [...activeSearchTerms];
    if (searchTerm && !terms.includes(searchTerm)) {
      terms.push(searchTerm);
    }
    
    // Process compound terms (e.g., with spaces or |)
    const processedTerms = [];
    // Keep track of column-specific search terms and their associated values
    const columnSearchTerms = {};
    
    terms.forEach(term => {
      const termLower = term.toLowerCase().trim();
      
      // Skip empty terms
      if (!termLower) return;
      
      // Handle prefixed terms with colon (e.g., "node:pve-01")
      if (termLower.includes(':')) {
        const [prefix, value] = termLower.split(':', 2);
        const prefixTrim = prefix.trim();
        
        // Add the original term
        processedTerms.push({
          term: termLower,
          isColumnSpecific: true,
          columnId: prefixTrim
        });
        
        // Store the value for this column for highlighting only in that column
        if (value && value.trim() !== '') {
          if (!columnSearchTerms[prefixTrim]) {
            columnSearchTerms[prefixTrim] = [];
          }
          columnSearchTerms[prefixTrim].push(value.trim());
        }
      } else if (termLower.split(' ').length > 1) {
        // Handle OR search with spaces
        const spaceTerms = termLower.split(' ').map(t => t.trim()).filter(t => t);
        spaceTerms.forEach(t => {
          processedTerms.push({
            term: t,
            isColumnSpecific: false
          });
        });
        
        // Also add the original term in case exact phrase matching is needed
        processedTerms.push({
          term: termLower,
          isColumnSpecific: false
        });
      } else if (termLower.includes('|')) {
        // Handle OR search with pipe character
        const orTerms = termLower.split('|').map(t => t.trim()).filter(t => t);
        orTerms.forEach(t => {
          processedTerms.push({
            term: t,
            isColumnSpecific: false
          });
        });
      } else {
        processedTerms.push({
          term: termLower,
          isColumnSpecific: false
        });
      }
    });
    
    // For numerical terms, remove leading zeros
    const finalTerms = processedTerms.map(item => {
      if (/^\d+$/.test(item.term)) {
        return {
          ...item,
          term: String(parseInt(item.term, 10))
        };
      }
      return item;
    });
    
    return { 
      terms: finalTerms, 
      columnSearchTerms 
    };
  }, [searchTerm, activeSearchTerms]);
  
  // Function to check if text contains any of the search terms
  const hasSearchMatch = (text, columnId) => {
    if (!text || !allSearchTerms || !allSearchTerms.terms || allSearchTerms.terms.length === 0) {
      return false;
    }
    
    // Convert to string to handle numeric values
    const textStr = String(text || '').toLowerCase();
    
    // Check for column-specific terms first
    if (columnId && allSearchTerms.columnSearchTerms && allSearchTerms.columnSearchTerms[columnId]) {
      // Look for matches from column-specific values
      if (allSearchTerms.columnSearchTerms[columnId].some(term => 
        textStr.includes(String(term).toLowerCase())
      )) {
        return true;
      }
    }
    
    // Check non-column-specific terms
    return allSearchTerms.terms
      .filter(item => !item.isColumnSpecific) // Only use generic terms
      .some(item => textStr.includes(String(item.term).toLowerCase()));
  };
  
  // Determine if the guest is running
  const isRunning = guest.status?.toLowerCase() === 'running';
  
  // Determine if this is a shared guest by checking if it actually exists on multiple nodes
  // First check if it's in the sharedGuestIds map, otherwise fall back to the ID format check
  const isSharedGuest = useMemo(() => {
    // For guests with VMID 999 or 888, show more detailed logging
    if (guest?.vmid === 999 || guest?.vmid === 888) {
      console.info('⚠️ Special VMID detected:', { 
        id: guest?.id, 
        vmid: guest?.vmid, 
        name: guest?.name,
        isInSharedMap: !!(guest?.id && sharedGuestIds[guest.id]),
        hasColonInId: !!(guest?.id && guest.id.includes(':')),
        hastate: guest?.hastate
      });
    }
    
    // Check if the guest has a meaningful HA state (not undefined, not ignored)
    // Only guests managed by HA or shared across nodes should show in the HA column
    const hasHaState = guest?.hastate && guest.hastate !== 'ignored';
    
    // Is the guest shared across nodes? (exists in sharedGuestIds map or has ':' in id)
    const isShared = (guest?.id && sharedGuestIds[guest.id]) || 
                     (guest?.id && guest.id.includes(':')) ||
                     !!guest?.shared;

    // For debugging: log about 1% of the guests to avoid flooding the console
    if (Math.random() < 0.01) {
      console.info('Guest HA Status Check:', { 
        id: guest?.id,
        name: guest.name,
        hastate: guest.hastate,
        isShared,
        hasHaState,
        shouldShowHaStatus: hasHaState || isShared
      });
    }

    // Return true if the guest is either HA-managed or shared
    return hasHaState || isShared;
  }, [guest?.id, guest?.vmid, guest?.name, guest?.hastate, guest?.shared, sharedGuestIds]);
  
  // If the guest is running, it's the primary node for this guest
  // In development mode, alternate primary/secondary for testing
  const isPrimaryNode = useMemo(() => {
    const isDevelopmentMode = localStorage.getItem('NODE_ENV') === 'development' || 
                            localStorage.getItem('use_mock_data') === 'true';
    
    // Use hastate field if available to determine primary status
    // Specifically for showing visuals, any guest with a hastate that isn't stopped
    // or disabled should be treated as "PRIMARY" (for the visual only)
    if (guest?.hastate) {
      if (guest.hastate === 'stopped' || guest.hastate === 'disabled') {
        return false;
      }
      // For any other state (started, error, fence, etc), show as primary
      return true;
    }
    
    // No hastate? Just use the running status directly
    return isRunning;
    
    // Previous logic - commented out as it was defaulting most guests to SECONDARY
    /*
    if (isDevelopmentMode && isSharedGuest) {
      // In dev mode, make some guests primary and others secondary for testing
      const guestIdNumber = parseInt(guest?.id?.split(':')[0] || guest?.id || '0', 10);
      return guestIdNumber % 2 === 0; // Even IDs are primary, odd are secondary
    }
    
    // Standard logic for production
    return isSharedGuest && isRunning;
    */
  }, [isRunning, guest?.hastate]);
  
  // Get metrics for this guest
  const cpuMetrics = metrics?.cpu?.[guest?.id] || null;
  const memoryMetrics = metrics?.memory?.[guest?.id] || null;
  const diskMetrics = metrics?.disk?.[guest?.id] || null;
  const networkMetrics = metrics?.network?.[guest?.id] || null;
  
  // Determine guest type
  const isVM = guest.type === 'qemu';
  const guestTypeLabel = isVM ? 'VM' : 'CT';
  const guestTypeColor = isVM ? 'primary' : 'secondary';
  
  // Get the CPU usage value
  const cpuUsage = cpuMetrics?.usage ?? 0;
  
  // Get memory usage with unit awareness
  const memoryUsage = memoryMetrics?.usagePercent ?? 0;
  const memoryUnit = memoryMetrics?.unit || 'bytes';
  const memoryTotal = memoryMetrics?.total ?? 0;
  const memoryUsed = memoryMetrics?.used ?? 0;
  
  // Get disk usage with unit awareness
  const diskUsage = diskMetrics?.usagePercent ?? 0;
  const diskUnit = diskMetrics?.unit || 'bytes';
  const diskTotal = diskMetrics?.total ?? 0;
  const diskUsed = diskMetrics?.used ?? 0;
  
  // Get network rates with unit awareness
  const downloadRate = networkMetrics?.inRate ?? 0;
  const uploadRate = networkMetrics?.outRate ?? 0;
  
  // Get uptime
  const uptime = guest.uptime ?? 0;
  
  // Define column groups for sizing strategy (same as in header)
  const fixedNarrowColumns = ['type', 'id', 'status']; // Very narrow columns
  const fixedWidthColumns = ['download', 'upload', 'uptime']; // Fixed width for network stats
  const autoSizeColumns = ['name', 'node']; // Auto-size to content
  const flexibleEqualColumns = ['cpu', 'memory', 'disk']; // Equal width with progress bars
  
  // Calculate visible columns and their groups
  const getVisibleColumnGroups = useMemo(() => {
    if (!columnOrder || !Array.isArray(columnOrder) || columnOrder.length === 0) {
      return { fixedNarrow: [], fixedWidth: [], autoSize: [], flexibleEqual: [] };
    }
    
    const visibleColumnIds = columnOrder.filter(id => columnVisibility[id]?.visible);
    
    return {
      fixedNarrow: visibleColumnIds.filter(id => fixedNarrowColumns.includes(id)),
      fixedWidth: visibleColumnIds.filter(id => fixedWidthColumns.includes(id)),
      autoSize: visibleColumnIds.filter(id => autoSizeColumns.includes(id)),
      flexibleEqual: visibleColumnIds.filter(id => flexibleEqualColumns.includes(id))
    };
  }, [columnOrder, columnVisibility]);
  
  // Get visible columns in the correct order
  const visibleColumns = useMemo(() => {
    if (!columnOrder || !Array.isArray(columnOrder) || columnOrder.length === 0) {
      return [];
    }
    
    // Filter visible columns and sort them according to columnOrder
    return columnOrder
      .filter(id => columnVisibility[id]?.visible)
      .map(id => ({ id, ...columnVisibility[id] }))
      .filter(Boolean);
  }, [columnVisibility, columnOrder]);

  // Helper function to get width for a column
  const getColumnWidth = (columnId) => {
    // Define fixed widths for specific columns
    const fixedWidths = {
      type: '50px',     // Just "VM" or "CT"
      id: '60px',       // Just numeric IDs
      status: '50px',   // Just the status circle
      role: '100px',    // Primary/Secondary chip
      download: '90px', // Network rates
      upload: '90px',   // Network rates
      uptime: '90px'    // Time display
    };
    
    // If column has a fixed width, return it
    if (fixedWidths[columnId]) {
      return fixedWidths[columnId];
    }
    
    // For auto-sized columns, use different strategies for name vs node
    if (autoSizeColumns.includes(columnId)) {
      // Use max-content for node to fit content more closely
      if (columnId === 'node') {
        return 'max-content';
      }
      // Use auto for name which can be longer
      return 'auto';
    }
    
    // For flexible equal columns (cpu, memory, disk), use percentage
    if (flexibleEqualColumns.includes(columnId)) {
      const visibleFlexColumns = getVisibleColumnGroups.flexibleEqual.length;
      
      if (visibleFlexColumns > 0) {
        // Use the same calculation as in the header
        return `${Math.floor(100 / visibleFlexColumns)}%`;
      }
      
      // Fallback if something goes wrong
      return '20%';
    }
    
    // Default fallback
    return 'auto';
  };

  // Check if a column is a flexible column
  const isFlexibleColumn = (columnId) => {
    return flexibleEqualColumns.includes(columnId);
  };

  // Function to check if a cell contains content that matches search terms
  const cellHasMatch = (columnId) => {
    if (!columnId) {
      return false;
    }

    // Check if this column has an active threshold filter
    if (['cpu', 'memory', 'disk', 'download', 'upload'].includes(columnId)) {
      // Debug log to see what filters are being passed
      if (columnId === 'cpu') {
        console.log('CPU Column Check:', { 
          columnId, 
          hasFilters: !!filters,
          filterValue: filters?.[columnId],
          isActive: filters && filters[columnId] > 0,
          filtersObj: filters ? {...filters} : null
        });
      }
      
      // Handle all possible ways to check for active filters
      // 1. Direct filters object check
      if (filters && filters[columnId] > 0) {
        return true;
      }
      
      // 2. Check the activeFilteredColumns object which might have a different structure
      if (activeFilteredColumns && activeFilteredColumns[columnId]) {
        return true;
      }
      
      // 3. Check if we have a direct match with the thresholdColumn
      if (thresholdColumn === columnId && thresholdValue > 0) {
        return true;
      }
      
      // 4. Check if we have a threshold search term in the search context
      const hasThresholdSearch = allSearchTerms?.terms?.some(item => 
        item.isColumnSpecific && 
        item.columnId === columnId && 
        item.term.includes('>') && 
        /\d/.test(item.term)
      );
      
      if (hasThresholdSearch) {
        return true;
      }
    }

    // Continue with regular search term matching if there are search terms
    if (!allSearchTerms || !allSearchTerms.terms || allSearchTerms.terms.length === 0) {
      return false;
    }

    // Check for column-specific search terms first
    const columnPrefixedTerms = allSearchTerms.terms.filter(item => {
      if (!item.isColumnSpecific) return false;
      return item.columnId === columnId;
    });

    // If we have column-specific terms targeting this column, it's a match
    if (columnPrefixedTerms.length > 0) {
      return true;
    }

    // Get the text content for this cell based on column ID
    let cellText;
    switch (columnId) {
      case 'node':
        cellText = getNodeName(guest?.node);
        break;
      case 'role':
        cellText = isSharedGuest ? 
          (guest?.hastate === 'started' ? 'primary started' :
           guest?.hastate === 'stopped' ? 'secondary stopped' :
           guest?.hastate === 'migrate' ? 'migrating migration' :
           guest?.hastate === 'relocate' ? 'relocating relocation' :
           guest?.hastate === 'error' ? 'error' :
           guest?.hastate === 'fence' ? 'fence fencing' :
           guest?.hastate === 'recovery' ? 'recovery recovering' :
           guest?.hastate === 'disabled' ? 'disabled' :
           isPrimaryNode ? 'primary' : 'secondary') 
          : (guest?.hastate && guest.hastate !== 'ignored' ? 'ha managed high-availability' : '');
        break;
      case 'type':
        cellText = isVM ? 'VM virtual machine' : 'CT container';
        break;
      case 'id':
        cellText = extractNumericId(guest?.id);
        break;
      case 'status':
        cellText = isRunning ? 'running' : 'stopped';
        break;
      case 'name':
        cellText = guest?.name;
        break;
      case 'cpu':
        cellText = `cpu ${formatPercentage(cpuUsage)}`;
        break;
      case 'memory':
        cellText = `memory ${formatBytesWithUnit(memoryUsed, memoryUnit)} ${formatBytesWithUnit(memoryTotal, memoryUnit)} ${memoryUsage}%`;
        break;
      case 'disk':
        cellText = `disk ${formatBytesWithUnit(diskUsed, diskUnit)} ${formatBytesWithUnit(diskTotal, diskUnit)} ${diskUsage}%`;
        break;
      case 'download':
        cellText = formatNetworkRate(downloadRate);
        break;
      case 'upload':
        cellText = formatNetworkRate(uploadRate);
        break;
      case 'uptime':
        cellText = isRunning ? formatUptime(uptime) : '—';
        break;
      default:
        cellText = '';
    }

    // Now check for general search terms that match this cell's content
    return hasSearchMatch(cellText, columnId);
  };

  // Render cell content based on column ID
  const renderCellContent = (columnId) => {
    if (!columnId) return null;
    
    switch (columnId) {
      case 'node':
        return (
          <Box sx={{ display: 'flex', alignItems: 'center' }}>
            <Typography 
              variant="body2" 
              noWrap
              sx={{
                fontWeight: isPrimaryNode ? 'bold' : 'normal',
                color: isPrimaryNode ? 'primary.main' : nodeTextColor,
              }}
            >
              {getNodeName(guest?.node)}
            </Typography>
          </Box>
        );
      
      case 'role':
        return (
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            {/* Only show HA status chip if one of these conditions is true:
                1. Guest has a meaningful hastate (not ignored and not undefined)
                2. Guest is shared among multiple nodes (truly HA-managed)
            */}
            {(guest?.hastate && guest.hastate !== 'ignored') || 
             (guest?.shared === true || (guest?.id && (sharedGuestIds[guest.id] || guest.id.includes(':')))) ? (
              <Tooltip title={
                guest?.hastate === 'started' ? "Primary Node - Currently running this shared guest" :
                guest?.hastate === 'stopped' ? "Secondary Node - This guest is running on another node" :
                guest?.hastate === 'migrate' ? "Migration in Progress - Guest is being moved to another node" :
                guest?.hastate === 'relocate' ? "Relocation in Progress - Guest is being cold-migrated" :
                guest?.hastate === 'error' ? "Error State - High availability issue detected" :
                guest?.hastate === 'fence' ? "Fencing Required - Node needs to be fenced to recover this guest" :
                guest?.hastate === 'recovery' ? "Recovery in Progress - Guest is being recovered" :
                guest?.hastate === 'disabled' ? "HA Disabled - Guest is not actively managed by HA" :
                guest?.hastate === 'ignored' ? "Not managed by High Availability" :
                isRunning ? "Primary Node - Currently running this guest" : 
                "Secondary Node - This guest is not running on this node"
              }>
                <Chip
                  size="small"
                  label={
                    guest?.hastate === 'started' ? "PRIMARY" :
                    guest?.hastate === 'stopped' ? "SECONDARY" :
                    guest?.hastate === 'migrate' ? "MIGRATING" :
                    guest?.hastate === 'relocate' ? "RELOCATING" :
                    guest?.hastate === 'error' ? "ERROR" :
                    guest?.hastate === 'fence' ? "FENCE" :
                    guest?.hastate === 'recovery' ? "RECOVERY" :
                    guest?.hastate === 'disabled' ? "DISABLED" :
                    guest?.hastate === 'ignored' ? "IGNORED" :
                    isRunning ? "PRIMARY" : "SECONDARY"
                  }
                  sx={{
                    height: '16px',
                    fontSize: '0.6rem',
                    backgroundColor: 
                      guest?.hastate === 'started' ? alpha(theme.palette.primary.main, 0.2) :
                      guest?.hastate === 'stopped' ? alpha(theme.palette.text.secondary, 0.1) :
                      guest?.hastate === 'migrate' ? alpha(theme.palette.info.main, 0.2) :
                      guest?.hastate === 'relocate' ? alpha(theme.palette.info.main, 0.2) :
                      guest?.hastate === 'error' ? alpha(theme.palette.error.main, 0.2) :
                      guest?.hastate === 'fence' ? alpha(theme.palette.warning.main, 0.2) :
                      guest?.hastate === 'recovery' ? alpha(theme.palette.warning.main, 0.2) :
                      guest?.hastate === 'disabled' ? alpha(theme.palette.text.disabled, 0.2) :
                      guest?.hastate === 'ignored' ? alpha(theme.palette.text.disabled, 0.1) :
                      isRunning ? alpha(theme.palette.primary.main, 0.2) : alpha(theme.palette.text.secondary, 0.1),
                    color: 
                      guest?.hastate === 'started' ? theme.palette.primary.main :
                      guest?.hastate === 'stopped' ? theme.palette.text.secondary :
                      guest?.hastate === 'migrate' ? theme.palette.info.main :
                      guest?.hastate === 'relocate' ? theme.palette.info.main :
                      guest?.hastate === 'error' ? theme.palette.error.main :
                      guest?.hastate === 'fence' ? theme.palette.warning.main :
                      guest?.hastate === 'recovery' ? theme.palette.warning.main :
                      guest?.hastate === 'disabled' ? theme.palette.text.disabled :
                      guest?.hastate === 'ignored' ? theme.palette.text.disabled :
                      isRunning ? theme.palette.primary.main : theme.palette.text.secondary,
                    borderColor: 
                      guest?.hastate === 'started' ? alpha(theme.palette.primary.main, 0.3) :
                      guest?.hastate === 'stopped' ? alpha(theme.palette.text.secondary, 0.2) :
                      guest?.hastate === 'migrate' ? alpha(theme.palette.info.main, 0.3) :
                      guest?.hastate === 'relocate' ? alpha(theme.palette.info.main, 0.3) :
                      guest?.hastate === 'error' ? alpha(theme.palette.error.main, 0.3) :
                      guest?.hastate === 'fence' ? alpha(theme.palette.warning.main, 0.3) :
                      guest?.hastate === 'recovery' ? alpha(theme.palette.warning.main, 0.3) :
                      guest?.hastate === 'disabled' ? alpha(theme.palette.text.disabled, 0.3) :
                      guest?.hastate === 'ignored' ? alpha(theme.palette.text.disabled, 0.2) :
                      isRunning ? alpha(theme.palette.primary.main, 0.3) : alpha(theme.palette.text.secondary, 0.2),
                  }}
                />
              </Tooltip>
            ) : null}
          </Box>
        );
        
      case 'type':
        return (
          <Box sx={{ 
            display: 'flex', 
            alignItems: 'center', 
            justifyContent: 'center',
            width: '100%',
            height: '100%'
          }}>
            <Tooltip title={isVM ? 'Virtual Machine' : 'Container'}>
              <Typography 
                variant="caption" 
                sx={{
                  fontWeight: 'bold',
                  color: isVM ? theme.palette.primary.main : '#00C853',
                  fontSize: '0.7rem'
                }}
              >
                {isVM ? 'VM' : 'CT'}
              </Typography>
            </Tooltip>
          </Box>
        );
      case 'id':
        return (
          <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
            {extractNumericId(guest?.id)}
          </Typography>
        );
      case 'status':
        return (
          <Box sx={{ 
            display: 'flex', 
            alignItems: 'center', 
            justifyContent: 'center',
            width: '100%',
            height: '100%'
          }}>
            {isSharedGuest ? (
              <Tooltip title={isPrimaryNode 
                ? `Primary: Running on ${nodeName}` 
                : `Secondary: Not running on ${nodeName}`
              }>
                <Box sx={{ display: 'flex', justifyContent: 'center' }}>
                  <StatusIndicator status={isRunning ? 'running' : 'stopped'} />
                </Box>
              </Tooltip>
            ) : (
              <Tooltip title={isRunning ? 'Running' : 'Stopped'}>
                <Box sx={{ display: 'flex', justifyContent: 'center' }}>
                  <StatusIndicator status={isRunning ? 'running' : 'stopped'} />
                </Box>
              </Tooltip>
            )}
          </Box>
        );
      case 'name':
        return (
          <Box sx={{ display: 'flex', alignItems: 'center' }}>
            <Typography variant="body2" noWrap>
              {guest?.name}
            </Typography>
          </Box>
        );
      case 'cpu':
        return (
          <ProgressWithLabel 
            value={cpuUsage} 
            label={`${formatPercentage(cpuUsage)}`} 
            color={cpuUsage > 90 ? 'error' : cpuUsage > 70 ? 'warning' : 'primary'}
          />
        );
      case 'memory':
        return (
          <ProgressWithLabel 
            value={memoryUsage} 
            label={`${formatBytesWithUnit(memoryUsed, memoryUnit)} / ${formatBytesWithUnit(memoryTotal, memoryUnit)}`} 
            color={memoryUsage > 90 ? 'error' : memoryUsage > 70 ? 'warning' : 'primary'}
          />
        );
      case 'disk':
        return (
          <ProgressWithLabel 
            value={diskUsage} 
            label={`${formatBytesWithUnit(diskUsed, diskUnit)} / ${formatBytesWithUnit(diskTotal, diskUnit)}`} 
            color={diskUsage > 90 ? 'error' : diskUsage > 70 ? 'warning' : 'primary'}
          />
        );
      case 'download':
        return (
          <Typography variant="body2" color="text.secondary">
            {formatNetworkRate(downloadRate)}
          </Typography>
        );
      case 'upload':
        return (
          <Typography variant="body2" color="text.secondary">
            {formatNetworkRate(uploadRate)}
          </Typography>
        );
      case 'uptime':
        return (
          <Typography variant="body2" color="text.secondary">
            {isRunning ? formatUptime(uptime) : '—'}
          </Typography>
        );
      default:
        return null;
    }
  };

  // Render the table row
  return (
    <TableRow 
      hover
      sx={{ 
        opacity: isRunning ? 1 : 0.6,
        '&:hover': {
          opacity: 1
        }
      }}
    >
      {/* Render cells based on column order */}
      {Array.isArray(visibleColumns) && visibleColumns.length > 0 ? (
        visibleColumns.map(column => {
          // Don't skip rendering the role column if user chose to make it visible
          // Only skip if user hasn't configured it and no cluster detected
          const roleColumnShouldDisplay = column.id !== 'role' || 
                                         column.visible === true || 
                                         localStorage.getItem('CLUSTER_DETECTED') === 'true';
          
          if (!roleColumnShouldDisplay) {
            return null;
          }
          
          // Check if this cell has a matching search term
          const hasMatch = cellHasMatch(column.id);
          
          // Special handling for node column to use custom text color
          const isNodeColumn = column.id === 'node';
          
          return (
            column && (
              <TableCell 
                key={column.id}
                sx={{ 
                  width: getColumnWidth(column.id),
                  minWidth: getMinWidthForColumn(column.id),
                  maxWidth: autoSizeColumns.includes(column.id) ? '300px' : 'none',
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  boxSizing: 'border-box',
                  // Use cell-level highlighting based on text match
                  backgroundColor: hasMatch 
                    ? alpha(theme.palette.primary.main, 0.15)
                    : 'inherit',
                  ...(fixedNarrowColumns.includes(column.id) && {
                    textAlign: 'center',
                    padding: getTableCellPadding(true)
                  }),
                  ...(!fixedNarrowColumns.includes(column.id) && {
                    padding: getTableCellPadding(false)
                  }),
                  // Special styling for node column
                  ...(isNodeColumn && {
                    '& .MuiTypography-root': {
                      color: nodeTextColor,
                      fontWeight: 500
                    }
                  })
                }}
              >
                {renderCellContent(column.id)}
              </TableCell>
            )
          );
        })
      ) : (
        <TableCell colSpan={2}>No visible columns</TableCell>
      )}
    </TableRow>
  );
};

// Helper function to get minimum width for each column
const getMinWidthForColumn = (columnId) => {
  const minWidths = {
    // Fixed narrow columns - add a few pixels to accommodate sort icon
    type: 50,      // Very small - just "VM" or "CT"
    id: 60,        // Very small - just numeric IDs
    status: 50,    // Minimal - just an icon
    role: 100,     // Primary/Secondary chip
    
    // Auto-sized columns
    name: 120,     // Names - minimum width
    node: 80,      // Node names - minimum width (reduced from 100)
    
    // Flexible equal columns
    cpu: 140,      // Medium - progress bar with percentage
    memory: 140,   // Progress bar with byte values
    disk: 140,     // Progress bar with byte values
    
    // Fixed width columns
    download: 90,  // Network rates
    upload: 90,    // Network rates
    uptime: 90     // Time display
  };
  
  return minWidths[columnId] || 80;
};

export default NetworkTableRow; 