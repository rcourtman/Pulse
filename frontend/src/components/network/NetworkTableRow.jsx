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
  thresholdValue
}) => {
  const theme = useTheme();
  
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
  
  // Determine if this is a shared guest (ID 999 or 888)
  const isSharedGuest = guest.vmid === 999 || guest.vmid === 888;
  
  // If the guest is running and it's a shared guest, it's on its primary node
  const isPrimaryNode = isSharedGuest && isRunning;
  
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
    if (!columnId || !allSearchTerms || !allSearchTerms.terms || allSearchTerms.terms.length === 0) {
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
            <Typography variant="body2" noWrap>
              {getNodeName(guest?.node)}
            </Typography>
            {isSharedGuest && isPrimaryNode && (
              <Tooltip title="Primary Node - This node is currently running this shared guest">
                <Box component="span" sx={{ 
                  display: 'inline-block',
                  width: '6px',
                  height: '6px',
                  borderRadius: '50%',
                  backgroundColor: 'primary.main',
                  opacity: 0.6,
                  ml: 1,
                  verticalAlign: 'middle'
                }} />
              </Tooltip>
            )}
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
          <Typography variant="body2" color="text.secondary">
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
            <Tooltip title={isRunning ? 'Running' : 'Stopped'}>
              <Box sx={{ display: 'flex', justifyContent: 'center' }}>
                <StatusIndicator status={isRunning ? 'running' : 'stopped'} />
              </Box>
            </Tooltip>
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