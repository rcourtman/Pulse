import React, { useEffect, useMemo } from 'react';
import {
  TableBody,
  TableRow,
  TableCell,
  Box,
  Typography,
  Chip,
  Button,
  Divider,
  alpha,
  useTheme
} from '@mui/material';
import NetworkCheckIcon from '@mui/icons-material/NetworkCheck';
import FilterAltIcon from '@mui/icons-material/FilterList';
import VisibilityOffIcon from '@mui/icons-material/VisibilityOff';
import NetworkTableRow from './NetworkTableRow';
import { getNodeTextColor } from '../../utils/colorUtils';

const NetworkTableBody = ({
  sortedAndFilteredData,
  guestData,
  metricsData,
  columnVisibility,
  getNodeName,
  extractNumericId,
  resetFilters,
  showStopped,
  setShowStopped,
  guestTypeFilter,
  resetColumnVisibility,
  forceUpdateCounter,
  columnOrder,
  activeFilteredColumns = {},
  thresholdColumn,
  thresholdValue
}) => {
  const theme = useTheme();
  
  // Check if any columns are visible
  const hasVisibleColumns = Object.values(columnVisibility).some(col => col.visible);
  
  // Force re-render when forceUpdateCounter changes
  useEffect(() => {
    if (forceUpdateCounter > 0) {
      console.log('NetworkTableBody re-rendering due to forceUpdateCounter:', forceUpdateCounter);
    }
  }, [forceUpdateCounter]);
  
  // Group guests by node
  const groupedGuests = useMemo(() => {
    if (!sortedAndFilteredData || !Array.isArray(sortedAndFilteredData)) {
      return [];
    }
    
    // Use a Map to preserve insertion order
    const nodeGroups = new Map();
    
    // Group guests by node
    sortedAndFilteredData.forEach(guest => {
      const nodeName = getNodeName(guest?.node) || 'Unknown';
      
      if (!nodeGroups.has(nodeName)) {
        nodeGroups.set(nodeName, []);
      }
      
      nodeGroups.get(nodeName).push(guest);
    });
    
    // Convert Map to array of objects
    return Array.from(nodeGroups.entries()).map(([nodeName, guests]) => ({
      nodeName,
      guests
    }));
  }, [sortedAndFilteredData, getNodeName]);
  
  // If no columns are visible, show a message
  if (!hasVisibleColumns) {
    return (
      <TableBody>
        <TableRow>
          <TableCell colSpan={12} align="center" sx={{ py: 8 }}>
            <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', py: 2 }}>
              <VisibilityOffIcon sx={{ fontSize: 48, color: 'text.secondary', mb: 2 }} />
              <Typography variant="h6" gutterBottom>
                No columns are visible
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                Use the column visibility button to show some columns
              </Typography>
              <Button 
                variant="outlined" 
                onClick={resetColumnVisibility}
                startIcon={<VisibilityOffIcon />}
              >
                Reset Columns
              </Button>
            </Box>
          </TableCell>
        </TableRow>
      </TableBody>
    );
  }

  // If no data matches the filters, show a message
  if (sortedAndFilteredData.length === 0) {
    return (
      <TableBody sx={{ '& tr:last-child td': { borderBottom: 0 } }}>
        <TableRow>
          <TableCell colSpan={12} align="center" sx={{ py: 8 }}>
            <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', py: 2 }}>
              <FilterAltIcon sx={{ fontSize: 48, color: 'text.secondary', mb: 2 }} />
              <Typography variant="h6" gutterBottom>
                No matching guests found
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                Try adjusting your filters or search terms
              </Typography>
              <Button 
                variant="outlined" 
                onClick={resetFilters}
                startIcon={<FilterAltIcon />}
              >
                Reset Filters
              </Button>
            </Box>
          </TableCell>
        </TableRow>
      </TableBody>
    );
  }
  
  // Calculate visible columns for node header colspan
  const visibleColumnCount = Object.values(columnVisibility).filter(col => col.visible).length;
  
  // Render the table with data grouped by node
  return (
    <TableBody sx={{ 
      '& tr:last-child td': { borderBottom: 0 },
      '& .MuiTableRow-root': { borderBottom: '1px solid', borderColor: 'divider' }
    }}>
      {groupedGuests.map((nodeGroup, groupIndex) => {
        const { nodeName, guests } = nodeGroup;
        
        return (
          <React.Fragment key={`node-${nodeName}`}>
            {/* Node Header/Divider */}
            <TableRow 
              sx={{ 
                bgcolor: theme => alpha(theme.palette.primary.main, 0.05),
                '&:hover': {
                  bgcolor: theme => alpha(theme.palette.primary.main, 0.08)
                }
              }}
            >
              <TableCell 
                colSpan={visibleColumnCount} 
                sx={{ 
                  py: 0.75,
                  px: 2,
                  borderBottom: '2px solid',
                  borderTop: groupIndex > 0 ? '2px solid' : 'none',
                  borderColor: theme => alpha(theme.palette.divider, 0.8)
                }}
              >
                <Box sx={{ 
                  display: 'flex', 
                  alignItems: 'center',
                  justifyContent: 'space-between'
                }}>
                  <Typography 
                    variant="subtitle2" 
                    sx={{ 
                      fontWeight: 600,
                      color: theme => {
                        const color = getNodeTextColor(nodeName, theme.palette.mode);
                        return color;
                      }
                    }}
                  >
                    NODE: {nodeName}
                  </Typography>
                  <Typography 
                    variant="caption" 
                    color="text.secondary"
                    sx={{ fontWeight: 500 }}
                  >
                    {guests.length} {guests.length === 1 ? 'guest' : 'guests'}
                  </Typography>
                </Box>
              </TableCell>
            </TableRow>
            
            {/* Guest Rows */}
            {guests.map(guest => (
              <NetworkTableRow
                key={guest.id}
                guest={guest}
                metrics={metricsData}
                columnVisibility={columnVisibility}
                getNodeName={getNodeName}
                extractNumericId={extractNumericId}
                columnOrder={columnOrder}
                activeFilteredColumns={activeFilteredColumns}
                thresholdColumn={thresholdColumn}
                thresholdValue={thresholdValue}
              />
            ))}
          </React.Fragment>
        );
      })}
    </TableBody>
  );
};

export default NetworkTableBody; 