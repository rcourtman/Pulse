import React, { useEffect, useMemo, useLayoutEffect } from 'react';
import useSocket from '../../hooks/useSocket';
import useFormattedMetrics from '../../hooks/useFormattedMetrics';
import useMockMetrics from '../../hooks/useMockMetrics';
import { useThemeContext } from '../../context/ThemeContext';
import { Box, CircularProgress, useTheme, Typography, Button, IconButton, Badge, Tooltip } from '@mui/material';
import ViewColumnIcon from '@mui/icons-material/ViewColumn';
import { createPortal } from 'react-dom';

// Import hooks
import {
  useNetworkFilters,
  useColumnManagement,
  usePopoverManagement,
  useNotifications,
  useSortManagement,
  useKeyboardShortcuts,
  useActiveFilteredColumns,
  useDataProcessing
} from './hooks';

// Import components
import ConnectionErrorDisplay from './ConnectionErrorDisplay';
import {
  NetworkHeader,
  NetworkPopovers,
  NetworkNotification,
  NetworkTable
} from './components';

import { useSearchContext } from '../../context/SearchContext';

const NetworkDisplay = ({ selectedNode = 'all' }) => {
  const { 
    isConnected, 
    guestData, 
    metricsData,
    processedMetricsData, 
    forceUpdateCounter: socketUpdateCounter,
    error,
    connectionStatus,
    reconnect,
    nodeData
  } = useSocket();
  
  // Debug logging
  console.log('NetworkDisplay - selectedNode:', selectedNode);
  console.log('NetworkDisplay - guestData:', guestData?.length || 0, 'guests');
  console.log('NetworkDisplay - nodeData:', nodeData?.length || 0, 'nodes');
  
  // Add useEffect to log detailed data for debugging
  React.useEffect(() => {
    if (guestData && guestData.length > 0) {
      console.log('Guest data sample:', guestData[0]);
      console.log('All guests:', guestData);
    } else {
      console.warn('No guest data available');
    }
    
    if (nodeData && nodeData.length > 0) {
      console.log('Node data sample:', nodeData[0]);
    } else {
      console.warn('No node data available');
    }
    
    if (metricsData && metricsData.length > 0) {
      console.log('Metrics data sample:', metricsData[0]);
    } else {
      console.warn('No metrics data available');
    }
  }, [guestData, nodeData, metricsData]);
  
  // Prepare availableNodes for dropdown
  const availableNodes = useMemo(() => {
    // Start with the "All Nodes" option
    const nodes = [
      { id: 'all', name: 'All Nodes', count: 0 }
    ];
    
    // Add nodes from the nodeData
    if (nodeData && nodeData.length > 0) {
      // Add each node
      nodeData.forEach(node => {
        nodes.push({
          id: node.id,
          name: node.name || node.id,
          count: 0
        });
      });
      
      // Count guests per node
      if (guestData && guestData.length > 0) {
        // Initialize counts
        const nodeCounts = {};
        nodes.forEach(node => {
          nodeCounts[node.id] = 0;
        });
        
        // Count guests for each node
        guestData.forEach(guest => {
          if (guest.node) {
            const normalizedNodeId = guest.node.replace('-', '');
            if (nodeCounts[normalizedNodeId] !== undefined) {
              nodeCounts[normalizedNodeId]++;
            }
            if (nodeCounts[guest.node] !== undefined) {
              nodeCounts[guest.node]++;
            }
          }
        });
        
        // Update counts in nodes array
        nodes.forEach(node => {
          if (node.id !== 'all') {
            node.count = nodeCounts[node.id] || 0;
          }
        });
        
        // Update the "All Nodes" count
        nodes[0].count = guestData.length;
      }
    }
    
    return nodes;
  }, [nodeData, guestData]);
  
  // Handle node selection change
  const handleNodeChange = (event) => {
    // Check if we can access window.location
    if (window && window.location) {
      try {
        // Create a URL with the selected node as a query parameter
        const url = new URL(window.location);
        const nodeValue = event.target.value;
        
        // Update the URL with the new node value
        if (nodeValue === 'all') {
          // Remove the node parameter if 'all' is selected
          url.searchParams.delete('node');
        } else {
          // Set the node parameter to the selected value
          url.searchParams.set('node', nodeValue);
        }
        
        // Update the browser history without reloading the page
        window.history.pushState({}, '', url);
        
        // Dispatch a custom event to notify App.jsx about the node change
        const nodeChangeEvent = new CustomEvent('nodeChange', {
          detail: { node: nodeValue }
        });
        window.dispatchEvent(nodeChangeEvent);
        
        console.log('Node selection changed to:', nodeValue);
      } catch (error) {
        console.error('Error updating URL with node selection:', error);
      }
    }
  };
  
  // Handle status selection change
  const handleStatusChange = (status) => {
    if (window && window.location) {
      try {
        const url = new URL(window.location);
        
        // Update the URL with the new status value
        if (status === null) {
          // Remove the status parameter if 'all' is selected
          url.searchParams.delete('status');
        } else if (status === false) {
          // Set the status parameter to 'running'
          url.searchParams.set('status', 'running');
        } else {
          // Set the status parameter to 'stopped'
          url.searchParams.set('status', 'stopped');
        }
        
        // Update the browser history without reloading the page
        window.history.pushState({}, '', url);
        
        // Update the status filter
        setShowStopped(status);
        
        console.log('Status filter changed to:', status === null ? 'all' : status === false ? 'running' : 'stopped');
      } catch (error) {
        console.error('Error updating URL with status selection:', error);
      }
    }
  };
  
  // Handle type selection change
  const handleTypeChange = (type) => {
    if (window && window.location) {
      try {
        const url = new URL(window.location);
        
        // Update the URL with the new type value
        if (type === 'all') {
          // Remove the type parameter if 'all' is selected
          url.searchParams.delete('type');
        } else {
          // Set the type parameter to the selected value
          url.searchParams.set('type', type);
        }
        
        // Update the browser history without reloading the page
        window.history.pushState({}, '', url);
        
        // Update the type filter
        setGuestTypeFilter(type);
        
        console.log('Type filter changed to:', type);
      } catch (error) {
        console.error('Error updating URL with type selection:', error);
      }
    }
  };
  
  // Check URL parameters on initial load
  useEffect(() => {
    if (window && window.location) {
      try {
        const urlParams = new URLSearchParams(window.location.search);
        
        // Check for status parameter
        const statusParam = urlParams.get('status');
        if (statusParam) {
          if (statusParam === 'running') {
            setShowStopped(false);
          } else if (statusParam === 'stopped') {
            setShowStopped(true);
          }
        }
        
        // Check for type parameter
        const typeParam = urlParams.get('type');
        if (typeParam && (typeParam === 'vm' || typeParam === 'ct')) {
          setGuestTypeFilter(typeParam);
        }
      } catch (error) {
        console.error('Error reading URL parameters:', error);
      }
    }
  }, []);
  
  // Use the formatted metrics hook
  const formattedMetrics = useFormattedMetrics(metricsData);
  
  // Use mock metrics for testing - but only if we're not using real mock data from the server
  const mockMetrics = useMockMetrics(guestData);
  
  // Use notification hook - MOVED UP before it's used in the migration event listener
  const {
    snackbarOpen,
    snackbarMessage,
    snackbarSeverity,
    handleSnackbarClose,
    showNotification
  } = useNotifications();
  
  // Listen for migration events and show notifications
  React.useEffect(() => {
    const handleMigrationEvent = (event) => {
      try {
        console.log('Migration event received in NetworkDisplay:', event.detail);
        
        const { guestId, guestName, fromNode, toNode } = event.detail;
        
        // Get node names for better display
        const fromNodeName = nodeData?.find(n => n.id === fromNode)?.name || fromNode;
        const toNodeName = nodeData?.find(n => n.id === toNode)?.name || toNode;
        
        // Show a notification about the migration
        showNotification(
          `Migration: ${guestName || 'Guest'} migrated from ${fromNodeName} to ${toNodeName}`,
          'info'
        );
        
        // Show browser notification if supported and permission granted
        if ('Notification' in window) {
          // Check if permission is already granted
          if (Notification.permission === 'granted') {
            showBrowserNotification(guestName || 'Guest', fromNodeName, toNodeName);
          } 
          // Ask for permission if not denied
          else if (Notification.permission !== 'denied') {
            Notification.requestPermission().then(permission => {
              if (permission === 'granted') {
                showBrowserNotification(guestName || 'Guest', fromNodeName, toNodeName);
              }
            });
          }
        }
        
        console.log(`Migration event: ${guestName} (${guestId}) migrated from ${fromNodeName} to ${toNodeName}`);
      } catch (error) {
        console.error('Error handling migration event:', error);
      }
    };
    
    // Function to show browser notification
    const showBrowserNotification = (guestName, fromNode, toNode) => {
      try {
        const notificationOptions = {
          body: `${guestName} migrated from ${fromNode} to ${toNode}`,
          icon: '/logo192.png',
          tag: 'migration-event',
          requireInteraction: false
        };
        
        new Notification('Guest Migration', notificationOptions);
      } catch (error) {
        console.error('Error showing browser notification:', error);
      }
    };
    
    // Add event listener
    window.addEventListener('migration', handleMigrationEvent);
    
    // Clean up
    return () => {
      window.removeEventListener('migration', handleMigrationEvent);
    };
  }, [nodeData, showNotification]);
  
  const combinedMetrics = useMemo(() => {
    // When using the mock data server, always use the processed metrics
    const useMockData = localStorage.getItem('use_mock_data') === 'true' || 
                        localStorage.getItem('MOCK_DATA_ENABLED') === 'true';
    
    // Create a safe default metrics object structure in case data is missing
    const safeDefaultMetrics = {
      cpu: {},
      memory: {},
      disk: {},
      network: {}
    };
    
    // Log the update for debugging
    if (socketUpdateCounter % 20 === 0) {
      console.log(`NetworkDisplay metrics refresh #${socketUpdateCounter}`, {
        useMockData,
        processedMetricsData: {
          cpu: Object.keys(processedMetricsData?.cpu || {}).length,
          memory: Object.keys(processedMetricsData?.memory || {}).length,
          disk: Object.keys(processedMetricsData?.disk || {}).length,
          network: Object.keys(processedMetricsData?.network || {}).length
        }
      });
    }
    
    if (useMockData) {
      // Use processedMetricsData which is updated more frequently
      return processedMetricsData || safeDefaultMetrics;
    }
    
    // Otherwise, fall back to the previous behavior
    const hasRealCpuMetrics = formattedMetrics?.cpu && Object.keys(formattedMetrics.cpu).length > 0;
    const hasRealMemoryMetrics = formattedMetrics?.memory && Object.keys(formattedMetrics.memory).length > 0;
    const hasRealDiskMetrics = formattedMetrics?.disk && Object.keys(formattedMetrics.disk).length > 0;
    const hasRealNetworkMetrics = formattedMetrics?.network && Object.keys(formattedMetrics.network).length > 0;
    
    // If we have real metrics, use them; otherwise, use mock metrics
    return {
      cpu: hasRealCpuMetrics ? formattedMetrics.cpu : (mockMetrics?.cpu || {}),
      memory: hasRealMemoryMetrics ? formattedMetrics.memory : (mockMetrics?.memory || {}),
      disk: hasRealDiskMetrics ? formattedMetrics.disk : (mockMetrics?.disk || {}),
      network: hasRealNetworkMetrics ? formattedMetrics.network : (mockMetrics?.network || {})
    };
  }, [formattedMetrics, mockMetrics, processedMetricsData, socketUpdateCounter]);
  
  // Log metrics updates with the counter to verify it's changing
  useEffect(() => {
    console.log(`Metrics update #${socketUpdateCounter}`, {
      combinedMetrics: {
        cpu: Object.keys(combinedMetrics.cpu).length,
        memory: Object.keys(combinedMetrics.memory).length,
        disk: Object.keys(combinedMetrics.disk).length,
        network: Object.keys(combinedMetrics.network).length
      }
    });
  }, [socketUpdateCounter, combinedMetrics]);
  
  const theme = useTheme();
  const { darkMode } = useThemeContext();
  
  // Use column management hook
  const {
    columnVisibility,
    columnOrder,
    setColumnOrder,
    columnMenuAnchorEl,
    openColumnMenu,
    forceUpdateCounter,
    toggleColumnVisibility,
    resetColumnVisibility,
    handleColumnMenuOpen,
    handleColumnMenuClose
  } = useColumnManagement(showNotification);
  
  // Use sort management hook
  const {
    sortConfig,
    requestSort
  } = useSortManagement();
  
  // Get search state from context instead of local state
  const { 
    searchTerm, 
    setSearchTerm, 
    activeSearchTerms, 
    addSearchTerm, 
    removeSearchTerm,
    clearSearchTerms
  } = useSearchContext();
  
  // Use the network filters hook but don't use its search state
  const {
    filters,
    setFilters,
    showStopped,
    setShowStopped,
    showFilters,
    setShowFilters,
    guestTypeFilter,
    setGuestTypeFilter,
    sliderDragging,
    updateFilter,
    handleSliderDragStart,
    handleSliderDragEnd,
    clearFilter,
    resetFilters,
    activeFilterCount
  } = useNetworkFilters();
  
  // Use popover management hook
  const {
    filterAnchorEl,
    openFilters,
    filterButtonRef,
    handleFilterButtonClick,
    handleCloseFilterPopover,
    closeAllPopovers
  } = usePopoverManagement();
  
  // Update resetFilters to also clear search terms from context
  const handleResetFilters = () => {
    resetFilters();
    clearSearchTerms();
  };
  
  // Use keyboard shortcuts hook with handleResetFilters instead of resetFilters
  useKeyboardShortcuts({
    openFilters,
    openColumnMenu,
    resetFilters: handleResetFilters,
    closeAllPopovers,
    showNotification
  });
  
  // Use data processing hook
  const {
    processedData,
    getNodeName,
    extractNumericId,
    getNodeFilteredGuests,
    formatPercentage,
    formatNetworkRateForFilter
  } = useDataProcessing({
    guestData,
    nodeData,
    sortConfig,
    filters,
    showStopped,
    selectedNode,
    guestTypeFilter,
    metricsData: combinedMetrics
  });
  
  // Add debug logging for sorting
  useEffect(() => {
    console.log('Current sort configuration:', sortConfig);
    console.log('Processed data after sorting:', processedData?.length || 0, 'items');
  }, [sortConfig, processedData]);
  
  // Use active filtered columns hook
  const activeFilteredColumns = useActiveFilteredColumns({
    filters,
    guestTypeFilter,
    showStopped,
    nodeData
  });
  
  // Add an effect to listen for search term events
  useEffect(() => {
    const handleSearchTermAction = (event) => {
      const { term, action } = event.detail;
      
      // Handle status: filters
      if (term.startsWith('status:')) {
        const status = term.split(':', 2)[1]?.trim();
        if (action === 'add') {
          if (status === 'running') {
            setShowStopped(false);
          } else if (status === 'stopped') {
            setShowStopped(true);
          }
        } else if (action === 'remove') {
          // Reset status filter
          setShowStopped(null);
        }
      }
      
      // Handle type: filters
      if (term.startsWith('type:')) {
        const type = term.split(':', 2)[1]?.trim();
        if (action === 'add') {
          if (type === 'vm' || type === 'qemu') {
            setGuestTypeFilter('vm');
          } else if (type === 'ct' || type === 'lxc' || type === 'container') {
            setGuestTypeFilter('ct');
          }
        } else if (action === 'remove') {
          // Reset type filter to "all"
          setGuestTypeFilter('all');
        }
      }
      
      // Handle node: filters
      if (term.startsWith('node:')) {
        const node = term.split(':', 2)[1]?.trim();
        if (action === 'add') {
          // Find the node in availableNodes
          const foundNode = availableNodes.find(n => 
            n.name.toLowerCase() === node || 
            n.id.toLowerCase() === node
          );
          
          if (foundNode) {
            // Dispatch a custom event to notify App.jsx about the node change
            const nodeChangeEvent = new CustomEvent('nodeChange', {
              detail: { node: foundNode.id }
            });
            window.dispatchEvent(nodeChangeEvent);
          }
        } else if (action === 'remove') {
          // Reset node filter to "all"
          const nodeChangeEvent = new CustomEvent('nodeChange', {
            detail: { node: 'all' }
          });
          window.dispatchEvent(nodeChangeEvent);
        }
      }
    };
    
    window.addEventListener('searchTermAction', handleSearchTermAction);
    
    return () => {
      window.removeEventListener('searchTermAction', handleSearchTermAction);
    };
  }, [setShowStopped, setGuestTypeFilter, availableNodes]);
  
  // State for portal root element
  const [portalRoot, setPortalRoot] = React.useState(null);

  // Use createPortal to place the column visibility button in the app header
  useLayoutEffect(() => {
    // Find the column visibility container in the app header
    const columnVisibilityContainer = document.getElementById('column-visibility-app-header');
    
    if (columnVisibilityContainer) {
      // Create a div for the portal if it doesn't exist yet
      let portalContainer = document.getElementById('column-visibility-portal');
      if (!portalContainer) {
        portalContainer = document.createElement('div');
        portalContainer.id = 'column-visibility-portal';
        // Clear previous content and append the portal container
        columnVisibilityContainer.innerHTML = '';
        columnVisibilityContainer.appendChild(portalContainer);
      }
      
      // Set the portal root for rendering
      setPortalRoot(portalContainer);
      
      // Clean up function
      return () => {
        setPortalRoot(null);
        if (portalContainer && portalContainer.parentNode) {
          portalContainer.parentNode.removeChild(portalContainer);
        }
      };
    }
  }, []);
  
  // Render the column visibility button in the portal
  const renderColumnVisibilityPortal = () => {
    if (!portalRoot) return null;
    
    // Count hidden columns for the badge
    const hiddenColumnsCount = Object.values(columnVisibility).filter(config => !config.visible).length;
    
    return createPortal(
      <Tooltip title={hiddenColumnsCount > 0 ? `Show hidden columns (${hiddenColumnsCount})` : "Column visibility settings"}>
        <IconButton
          onClick={handleColumnMenuOpen}
          color={openColumnMenu ? 'primary' : 'default'}
          size="small"
          aria-controls={openColumnMenu ? 'column-menu' : undefined}
          aria-haspopup="true"
          aria-expanded={openColumnMenu ? 'true' : undefined}
          sx={{ 
            borderRadius: 1,
            p: 0.5,
            bgcolor: hiddenColumnsCount > 0 ? 'rgba(255, 255, 255, 0.1)' : 'transparent',
            color: 'white',  // Make sure the icon is visible in the dark app bar
            '&:hover': {
              bgcolor: 'rgba(255, 255, 255, 0.2)'
            }
          }}
        >
          <Badge
            badgeContent={hiddenColumnsCount}
            color="error"
            invisible={hiddenColumnsCount === 0}
          >
            <ViewColumnIcon />
          </Badge>
        </IconButton>
      </Tooltip>,
      portalRoot
    );
  };
  
  // Show loading state if not connected
  if (!isConnected) {
    if (error || connectionStatus === 'disconnected' || connectionStatus === 'error') {
      // Show connection error
      return (
        <ConnectionErrorDisplay 
          connectionStatus={connectionStatus} 
          error={error} 
          onReconnect={reconnect}
        />
      );
    }
  }
  
  // If we have no data but we're connected, show a message
  if (isConnected && (!guestData || guestData.length === 0)) {
    return (
      <Box sx={{ display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center', height: '50vh' }}>
        <Typography variant="h5" gutterBottom>No guest data available</Typography>
        <Typography variant="body1" color="text.secondary">
          The application is connected but no guest data was received.
        </Typography>
        <Button 
          variant="contained" 
          color="primary" 
          sx={{ mt: 2 }}
          onClick={() => {
            if (isConnected) {
              // Request fresh data
              reconnect();
            }
          }}
        >
          Refresh Data
        </Button>
      </Box>
    );
  }
  
  return (
    <Box sx={{ width: '100%' }}>
      {/* Render the column visibility button portal */}
      {renderColumnVisibilityPortal()}
      
      {/* Column Menu - Ensure it's still rendered to handle the actual menu */}
      <Box sx={{ position: 'absolute', top: 0, left: 0, visibility: 'hidden' }}>
        {openColumnMenu && (
          <span 
            ref={(node) => {
              if (node && !columnMenuAnchorEl) {
                handleColumnMenuOpen({ currentTarget: node });
              }
            }} 
          />
        )}
      </Box>
      
      {/* Popovers */}
      <NetworkPopovers
        // Filter popover props
        filterAnchorEl={filterAnchorEl}
        openFilters={openFilters}
        handleCloseFilterPopover={handleCloseFilterPopover}
        filters={filters}
        updateFilter={updateFilter}
        handleSliderDragStart={handleSliderDragStart}
        handleSliderDragEnd={handleSliderDragEnd}
        resetFilters={handleResetFilters}
        
        // Formatters
        formatPercentage={formatPercentage}
        formatNetworkRateForFilter={formatNetworkRateForFilter}
      />
      
      {/* Main data table */}
      <NetworkTable
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
        sortedAndFilteredData={processedData}
        guestData={guestData}
        metricsData={combinedMetrics}
        getNodeName={getNodeName}
        extractNumericId={extractNumericId}
        resetFilters={resetFilters}
        showStopped={showStopped}
        setShowStopped={setShowStopped}
        guestTypeFilter={guestTypeFilter}
        setGuestTypeFilter={setGuestTypeFilter}
        availableNodes={availableNodes}
        selectedNode={selectedNode}
        handleNodeChange={handleNodeChange}
        handleStatusChange={handleStatusChange}
        handleTypeChange={handleTypeChange}
        // Pass filter-related props
        filters={filters}
        updateFilter={updateFilter}
        handleFilterButtonClick={handleFilterButtonClick}
        filterButtonRef={filterButtonRef}
        openFilters={openFilters}
        handleCloseFilterPopover={handleCloseFilterPopover}
      />
      
      {/* Notification Snackbar */}
      <NetworkNotification
        snackbarOpen={snackbarOpen}
        snackbarMessage={snackbarMessage}
        snackbarSeverity={snackbarSeverity}
        handleSnackbarClose={handleSnackbarClose}
      />
    </Box>
  );
};

export default NetworkDisplay; 