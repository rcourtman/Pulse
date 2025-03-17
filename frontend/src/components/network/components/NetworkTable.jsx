import React, { useState, useEffect, useMemo } from 'react';
import {
  Card,
  CardContent,
  TableContainer,
  Table,
  Paper,
  Box,
  alpha,
  useTheme
} from '@mui/material';
import NetworkTableHeader from '../NetworkTableHeader';
import NetworkTableBody from '../NetworkTableBody';
import { STORAGE_KEY_COLUMN_VISIBILITY } from '../../../constants/networkConstants';
import axios from 'axios';

const NetworkTable = ({
  sortConfig,
  requestSort,
  columnVisibility,
  toggleColumnVisibility,
  resetColumnVisibility,
  columnMenuAnchorEl,
  handleColumnMenuOpen,
  handleColumnMenuClose,
  openColumnMenu,
  forceUpdateCounter,
  columnOrder,
  setColumnOrder,
  activeFilteredColumns,
  sortedAndFilteredData,
  guestData,
  metricsData,
  getNodeName,
  extractNumericId,
  resetFilters,
  showStopped,
  setShowStopped,
  guestTypeFilter,
  setGuestTypeFilter,
  availableNodes = [],
  selectedNode = 'all',
  handleNodeChange = () => {},
  handleStatusChange = () => {},
  handleTypeChange = () => {},
  filters = {},
  updateFilter = () => {},
  handleFilterButtonClick = () => {},
  filterButtonRef = null,
  openFilters = false,
  handleCloseFilterPopover = () => {},
  sharedGuestIdMap = {},
  updateRoleColumnVisibility = () => {}
}) => {
  const theme = useTheme();
  
  // Table container ref
  const tableContainerRef = React.useRef(null);
  const tableRef = React.useRef(null);

  // Get the column index from the column order
  const getColumnIndex = (columnId) => {
    // First get visible columns in the right order
    const visibleColumnIds = columnOrder.filter(id => columnVisibility[id]?.visible);
    return visibleColumnIds.indexOf(columnId);
  };

  // Add useMemo to detect cluster mode from actual node data
  const isClusterPresent = useMemo(() => {
    // Get all nodes from the current guest data
    const nodesInGuestData = guestData?.map(guest => guest.node) || [];
    const uniqueNodes = [...new Set(nodesInGuestData)];
    
    // If we have more than one node or if any guest has a hastate
    const hasHaStates = guestData?.some(guest => 
      guest.hastate && 
      guest.hastate !== 'ignored' && 
      guest.hastate !== '-'
    );
    
    // Check if any guests are marked as shared - another strong indicator of cluster mode
    const hasSharedGuests = guestData?.some(guest => guest.shared === true);
    
    // Check for HA resource data by looking for guests with ':' in their ID (sid format)
    const hasHaResourceFormat = guestData?.some(guest => 
      guest.id && typeof guest.id === 'string' && guest.id.includes(':')
    );
    
    // Check if shared guest map has any entries
    const hasSharedGuestMapEntries = Object.keys(sharedGuestIdMap || {}).length > 0;
    
    // Log what we're detecting for debugging
    console.log('Cluster detection from guest data:', {
      uniqueNodes,
      nodeCount: uniqueNodes.length,
      hasHaStates,
      hasSharedGuests,
      hasHaResourceFormat,
      hasSharedGuestMapEntries,
      haStatesFound: guestData?.filter(g => g.hastate && g.hastate !== 'ignored').map(g => g.hastate) || []
    });
    
    // Store the result in localStorage for persistence across page loads
    const isCluster = uniqueNodes.length > 1 || hasHaStates || hasSharedGuests || hasHaResourceFormat || hasSharedGuestMapEntries;
    
    // Update localStorage - this will trigger the detection in useColumnManagement
    const previousValue = localStorage.getItem('CLUSTER_DETECTED');
    if (previousValue !== isCluster.toString()) {
      console.log('Updating cluster detection state:', isCluster);
      localStorage.setItem('CLUSTER_DETECTED', isCluster.toString());
    }
    
    return isCluster;
  }, [guestData, sharedGuestIdMap]);
  
  // Check if we're in mock data mode
  const isMockData = useMemo(() => {
    // Check explicit mock data flags first
    const explicitMockEnabled = localStorage.getItem('MOCK_DATA_ENABLED') === 'true' || 
                               localStorage.getItem('use_mock_data') === 'true';
    
    // Only consider localhost as mock data if no explicit setting exists
    // This allows us to connect to real Proxmox on localhost without triggering mock mode
    const isOnLocalhost = window.location.hostname === 'localhost';
    
    return explicitMockEnabled;
  }, []);
  
  // Make a separate debug effect to constantly log status
  useEffect(() => {
    console.log('CURRENT COLUMN VISIBILITY STATUS:', {
      roleColumnVisible: columnVisibility?.role?.visible,
      clusterDetected: localStorage.getItem('CLUSTER_DETECTED') === 'true',
      isClusterPresent,
      isMockData
    });
  }, [columnVisibility, isClusterPresent, isMockData]);

  // Add immediate API check when component mounts
  useEffect(() => {
    // Only run once on initial mount
    const checkClusterStatusDirectly = async () => {
      try {
        console.log('Directly checking cluster status from API...');
        
        // Check if we're in mock data mode
        const isMockData = localStorage.getItem('MOCK_DATA_ENABLED') === 'true' || 
                          localStorage.getItem('use_mock_data') === 'true';
        
        // For production, check cluster status from the API
        const response = await axios.get('/api/cluster-status');
        const isCluster = response?.data?.clusterEnabled || false;
        console.log('API returned cluster status:', isCluster);
        
        // Update localStorage and column visibility
        localStorage.setItem('CLUSTER_DETECTED', isCluster.toString());
        
        // Always enforce the server-reported cluster status for column visibility
        // Even in mock data mode, respect the actual cluster status from the server
        if (typeof updateRoleColumnVisibility === 'function') {
          updateRoleColumnVisibility(isCluster);
          console.log(`HA Status column visibility set to ${isCluster} based on API response`);
        }
        
        // If not a cluster, force hide the column to ensure it's not visible
        if (!isCluster && columnVisibility?.role?.visible) {
          console.log('Server confirmed no cluster, forcing HA Status column to hide');
          toggleColumnVisibility('role');
        }
      } catch (error) {
        console.error('Error checking cluster status:', error);
        
        // Use client-side detection as fallback
        if (isClusterPresent) {
          localStorage.setItem('CLUSTER_DETECTED', 'true');
          if (typeof updateRoleColumnVisibility === 'function') {
            updateRoleColumnVisibility(true);
          }
        } else {
          localStorage.setItem('CLUSTER_DETECTED', 'false');
          if (typeof updateRoleColumnVisibility === 'function') {
            updateRoleColumnVisibility(false);
          }
        }
      }
    };
    
    // Call immediately 
    checkClusterStatusDirectly();
  }, [isClusterPresent, updateRoleColumnVisibility, columnVisibility, toggleColumnVisibility]);

  return (
    <Card elevation={0} sx={{ borderRadius: 2, overflow: 'hidden', position: 'relative' }}>
      <CardContent sx={{ 
        p: 0, 
        position: 'relative',
        overflow: 'hidden',
        '&:last-child': { pb: 0 }
      }}>
        <TableContainer 
          component={Paper} 
          elevation={0} 
          sx={{ 
            borderRadius: 0, 
            position: 'relative', 
            overflow: 'hidden'
          }}
          ref={tableContainerRef}
        >
          <Table 
            size="small" 
            aria-label="systems table" 
            sx={{ 
              tableLayout: 'auto', 
              width: '100%',
              '& .MuiTableCell-root': {
                borderBottom: '1px solid',
                borderColor: 'divider'
              }
            }}
            ref={tableRef}
          >
            <NetworkTableHeader
              sortConfig={sortConfig}
              requestSort={requestSort}
              columnVisibility={columnVisibility}
              toggleColumnVisibility={toggleColumnVisibility}
              resetColumnVisibility={resetColumnVisibility}
              columnMenuAnchorEl={columnMenuAnchorEl}
              handleColumnMenuOpen={handleColumnMenuOpen}
              handleColumnMenuClose={handleColumnMenuClose}
              openColumnMenu={openColumnMenu}
              forceUpdateCounter={forceUpdateCounter}
              columnOrder={columnOrder}
              setColumnOrder={setColumnOrder}
              activeFilteredColumns={activeFilteredColumns}
              showStopped={showStopped}
              setShowStopped={setShowStopped}
              guestTypeFilter={guestTypeFilter}
              setGuestTypeFilter={setGuestTypeFilter}
              availableNodes={availableNodes}
              selectedNode={selectedNode}
              handleNodeChange={handleNodeChange}
              handleStatusChange={handleStatusChange}
              handleTypeChange={handleTypeChange}
              filters={filters}
              updateFilter={updateFilter}
              handleFilterButtonClick={handleFilterButtonClick}
              filterButtonRef={filterButtonRef}
              openFilters={openFilters}
              handleCloseFilterPopover={handleCloseFilterPopover}
            />
            <NetworkTableBody
              sortedAndFilteredData={sortedAndFilteredData}
              guestData={guestData}
              metricsData={metricsData}
              getNodeName={getNodeName}
              extractNumericId={extractNumericId}
              columnVisibility={columnVisibility}
              columnOrder={columnOrder}
              activeFilteredColumns={activeFilteredColumns}
              sharedGuestIdMap={sharedGuestIdMap}
              filters={filters}
            />
          </Table>
        </TableContainer>
      </CardContent>
    </Card>
  );
};

export default NetworkTable; 