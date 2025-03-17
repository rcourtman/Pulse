import React, { useEffect, useMemo, useLayoutEffect, useState } from 'react';
import useSocket from '../../hooks/useSocket';
import useFormattedMetrics from '../../hooks/useFormattedMetrics';
import useMockMetrics from '../../hooks/useMockMetrics';
import { useThemeContext } from '../../context/ThemeContext';
import { Box, CircularProgress, useTheme, Typography, Button, IconButton, Badge, Tooltip } from '@mui/material';
import ViewColumnIcon from '@mui/icons-material/ViewColumn';
import { createPortal } from 'react-dom';
import { STORAGE_KEY_COLUMN_VISIBILITY } from '../../constants/networkConstants';

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
      
      // Track assignments per node
      const assignmentsByNode = {};
      
      guestData.forEach(guest => {
        const nodeId = guest.node;
        if (!assignmentsByNode[nodeId]) {
          assignmentsByNode[nodeId] = [];
        }
        assignmentsByNode[nodeId].push(`${guest.id} (${guest.name})`);
      });
      
      console.log('GUESTS BY NODE:');
      Object.keys(assignmentsByNode).sort().forEach(nodeId => {
        console.log(`Node "${nodeId}": ${assignmentsByNode[nodeId].length} guests`);
        console.log(`  Guests: ${assignmentsByNode[nodeId].join(', ')}`);
      });
    } else {
      console.warn('No guest data available');
    }
    
    if (nodeData && nodeData.length > 0) {
      console.log('Node data sample:', nodeData[0]);
      console.log('All nodes:', nodeData.map(n => ({ id: n.id, name: n.name })));
    } else {
      console.warn('No node data available');
    }
    
    if (metricsData && metricsData.length > 0) {
      console.log('Metrics data sample:', metricsData[0]);
    } else {
      console.warn('No metrics data available');
    }
  }, [guestData, nodeData, metricsData]);
  
  // Use notification hook - MOVED UP before it's used in the migration event listener
  const {
    snackbarOpen,
    snackbarMessage,
    snackbarSeverity,
    handleSnackbarClose,
    showNotification
  } = useNotifications();
  
  // Use column management hook - MOVED UP before it's used in availableNodes
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
    handleColumnMenuClose,
    updateRoleColumnVisibility
  } = useColumnManagement(showNotification);
  
  // Define sharedGuestIdMap at component level
  const [sharedGuestIdMap, setSharedGuestIdMap] = useState({});
  
  // Prepare availableNodes for dropdown
  const availableNodes = useMemo(() => {
    // Start with the "All Nodes" option
    const nodes = [
      { id: 'all', name: 'All Nodes', count: 0 }
    ];
    
    console.log('🔄 Building node dropdown list...');
    console.log('  nodeData length:', nodeData?.length || 0);
    console.log('  guestData length:', guestData?.length || 0);
    
    if (nodeData) {
      console.log('  nodeData sample:', nodeData?.slice(0, 3));
    }
    
    // Add fallback nodes if the backend isn't providing any
    // This ensures the dropdown always has nodes to select from
    let nodeList = nodeData;
    
    // If nodeData is empty or doesn't exist, generate fallback nodes based on the guest data
    if (!nodeData || nodeData.length === 0) {
      console.warn('🚨 No nodes received from backend, generating fallback nodes from guest data');
      
      if (guestData && guestData.length > 0) {
        // Collect all unique node names from guests
        const nodeSet = new Set();
        guestData.forEach(guest => {
          if (guest.node) {
            nodeSet.add(guest.node);
          }
        });
        
        // Create node objects from the unique node names
        nodeList = Array.from(nodeSet).map(nodeName => ({
          id: nodeName,
          name: nodeName
        }));
        
        console.log('🔄 Created fallback nodes:', nodeList);
      } else {
        // Last resort - create hardcoded fallback nodes
        nodeList = [
          { id: 'pve-prod-01', name: 'pve-prod-01' },
          { id: 'pve-prod-02', name: 'pve-prod-02' },
          { id: 'pve-dev-01', name: 'pve-dev-01' }
        ];
        console.log('🔄 Created hardcoded fallback nodes as last resort');
      }
    }
    
    // Add nodes from the nodeData
    if (nodeList && nodeList.length > 0) {
      console.log('🔄 Adding individual nodes to dropdown...');
      
      // Add each node
      nodeList.forEach(node => {
        if (!node || !node.id) {
          console.error('Invalid node in nodeData:', node);
          return;
        }
        console.log(`  Adding node: ${node.name || node.id}`);
        nodes.push({
          id: node.id,
          name: node.name || node.id,
          count: 0
        });
      });
      
      // Count guests per node
      if (guestData && guestData.length > 0) {
        // Look for guests with the same ID assigned to different nodes
        const guestIdToNodes = {};
        guestData.forEach(guest => {
          if (!guest.id) return;
          
          if (!guestIdToNodes[guest.id]) {
            guestIdToNodes[guest.id] = new Set();
          }
          guestIdToNodes[guest.id].add(guest.node || 'unknown');
        });
        
        // Check for shared guest IDs (same ID on multiple nodes)
        const sharedGuestIds = Object.entries(guestIdToNodes)
          .filter(([_, nodes]) => nodes.size > 1)
          .map(([id, nodes]) => ({
            id,
            nodes: Array.from(nodes)
          }));
        
        if (sharedGuestIds.length > 0) {
          console.warn('⚠️ Found guests shared across multiple nodes:', sharedGuestIds);
          console.warn('⚠️ This may cause incorrect guest counts in the dropdown menu');
          console.warn('⚠️ Total shared guests: ' + sharedGuestIds.length);
        }
        
        // Auto-hide role column if no shared guests are detected
        const areAnyNodesInCluster = nodeData?.some(node => node.isInCluster === true);
        
        // Check if we're in mock data mode with cluster enabled
        const isMockDataMode = localStorage.getItem('MOCK_DATA_ENABLED') === 'true' || 
                             import.meta.env.VITE_MOCK_DATA_ENABLED === 'true' ||
                             window.location.hostname === 'localhost';
        
        const isMockClusterMode = isMockDataMode;
        
        // Always show role column in mock data mode
        console.log('FORCING CLUSTER DETECTION:', {
          isMockDataMode,
          hostname: window.location.hostname,
          mockClusterEnabled: localStorage.getItem('MOCK_CLUSTER_ENABLED'),
          viteClusterEnabled: import.meta.env.VITE_MOCK_CLUSTER_ENABLED
        });
        
        // Debug log cluster mode detection
        console.log('DEBUG MOCK CLUSTER MODE:', {
          mockDataEnabled: localStorage.getItem('MOCK_DATA_ENABLED'),
          autoDetectCluster: localStorage.getItem('PROXMOX_AUTO_DETECT_CLUSTER'),
          mockClusterEnabled: localStorage.getItem('MOCK_CLUSTER_ENABLED'),
          viteClusterEnabled: import.meta.env.VITE_MOCK_CLUSTER_ENABLED,
          isMockDataMode,
          isMockClusterMode
        });
        
        console.info('⚠️ Cluster detection:', {
          nodesInCluster: areAnyNodesInCluster,
          mockClusterMode: isMockClusterMode,
          nodeData: nodeData?.map(n => ({ name: n.name, isInCluster: n.isInCluster })),
          sharedGuestsCount: sharedGuestIds.length
        });
        
        // Force column to be visible in development mode
        let shouldShowRoleColumn = isMockDataMode || areAnyNodesInCluster || sharedGuestIds.length > 0;
        
        // If we're using mock data, we should always show the role column
        if (isMockDataMode) {
          console.log('Mock data mode detected - forcing role column to be visible');
          localStorage.setItem('CLUSTER_DETECTED', 'true');
          shouldShowRoleColumn = true;
        }
        
        // Only show role column if either nodes are in a cluster or shared guests are detected
        updateRoleColumnVisibility(shouldShowRoleColumn);
        
        // Convert sharedGuestIds array to a map for easier lookup
        const newSharedGuestIdMap = sharedGuestIds.reduce((acc, item) => {
          acc[item.id] = item.nodes;
          return acc;
        }, {});
        
        // Update the shared guest map state
        setSharedGuestIdMap(newSharedGuestIdMap);
        
        // Debug actual counts directly from guestData
        const directCounts = {};
        guestData.forEach(guest => {
          const nodeId = guest.node;
          if (!nodeId) return;  // Skip guests with no node
          
          if (!directCounts[nodeId]) {
            directCounts[nodeId] = 0;
          }
          directCounts[nodeId]++;
        });
        
        console.log('🔄 Direct guest counts from guestData:');
        Object.keys(directCounts).sort().forEach(nodeId => {
          console.log(`  Node "${nodeId}": ${directCounts[nodeId]} guests`);
        });
        
        // Update counts directly from the directCounts object
        nodes.forEach(node => {
          if (node.id !== 'all') {
            node.count = directCounts[node.id] || 0;
            console.log(`🔄 Node dropdown: ${node.name} (${node.id}) has ${node.count} guests`);
            
            // Check for discrepancy between count and expected 10 guests
            if (node.count !== 10) {
              console.warn(`⚠️ Node ${node.id} has ${node.count} guests, expected 10!`);
              
              // List all guests for this node to help diagnose
              const nodeGuests = guestData.filter(g => g.node === node.id);
              console.log(`  Guests for ${node.id}:`, nodeGuests.map(g => g.id));
            }
          }
        });
        
        // Update the "All Nodes" count
        nodes[0].count = guestData.length;
      }
    } else {
      console.error('🚨 No nodes found in nodeData! This will result in an empty dropdown.');
    }
    
    // Log the final node dropdown items
    console.log('🔄 Final node dropdown list:', JSON.stringify(nodes));
    return nodes;
  }, [nodeData, guestData, updateRoleColumnVisibility]);
  
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
        
        // Check mock data params from server and store in localStorage for the role column
        // The goal is to transfer server-side configuration to localStorage for frontend to access
        if (window.SERVER_CONFIG) {
          console.log('Found server config, updating localStorage with server settings');
          if (window.SERVER_CONFIG.MOCK_DATA_ENABLED) {
            localStorage.setItem('MOCK_DATA_ENABLED', 'true');
          }
          if (window.SERVER_CONFIG.PROXMOX_AUTO_DETECT_CLUSTER) {
            localStorage.setItem('PROXMOX_AUTO_DETECT_CLUSTER', 'true');
          }
          if (window.SERVER_CONFIG.NODE_ENV) {
            localStorage.setItem('NODE_ENV', window.SERVER_CONFIG.NODE_ENV);
          }
        }
        
        // Check for server environment from document to detect development mode
        if (document.body.classList.contains('development-mode')) {
          localStorage.setItem('environment', 'development');
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
  
  // Detect if we're in a mock/dev environment
  useEffect(() => {
    // Check for development mode
    if (process.env.NODE_ENV === 'development') {
      localStorage.setItem('NODE_ENV', 'development');
    }
    
    // Check for mock data indicators
    if (localStorage.getItem('use_mock_data') !== 'true') {
      // Check if there's mock in the URL
      const isMockInUrl = window.location.href.toLowerCase().includes('mock');
      
      // Check if there's localhost in the URL
      const isLocalhost = window.location.hostname === 'localhost' || 
                          window.location.hostname === '127.0.0.1';
      
      // If either condition is true, mark as using mock data
      if (isMockInUrl || isLocalhost) {
        localStorage.setItem('use_mock_data', 'true');
      }
    }
  }, []);
  
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
    metricsData: processedMetricsData
  });
  
  // Add debugging right before render to see what's actually being displayed
  useEffect(() => {
    if (processedData && processedData.length > 0) {
      console.log('🔍 FINAL DATA BEING DISPLAYED:', processedData.length, 'guests');
      
      // Group by node for display
      const nodeGroups = {};
      processedData.forEach(guest => {
        if (!guest) return;
        const nodeName = guest.node || 'unknown';
        if (!nodeGroups[nodeName]) {
          nodeGroups[nodeName] = [];
        }
        nodeGroups[nodeName].push(guest);
      });
      
      console.log('🔍 FINAL NODE DISTRIBUTION:');
      Object.keys(nodeGroups).sort().forEach(nodeName => {
        const count = nodeGroups[nodeName]?.length || 0;
        const highlight = count < 10 ? '⚠️' : '✅';
        console.log(`${highlight} Node "${nodeName}": ${count} guests`);
        
        // Log the actual guests for problematic nodes (less than 10)
        if (count < 10) {
          console.log(`  Guest IDs: ${nodeGroups[nodeName].map(g => g.id).join(', ')}`);
          
          // Check if these guests have matching node properties
          console.log(`  Node properties check:`);
          nodeGroups[nodeName].forEach(guest => {
            console.log(`    ${guest.id} (${guest.name}): node=${guest.node}, nodeId=${guest.nodeId || 'N/A'}`);
          });
        }
      });
      
      // Check if we're missing any guests from our node filters
      if (guestData) {
        const allNodeCounts = {};
        guestData.forEach(guest => {
          if (!guest) return;
          const node = guest.node || 'unknown';
          allNodeCounts[node] = (allNodeCounts[node] || 0) + 1;
        });
        
        // Check for discrepancies between raw data and filtered data
        Object.keys(allNodeCounts).forEach(node => {
          const rawCount = allNodeCounts[node] || 0;
          const filteredCount = (nodeGroups[node] || []).length;
          
          if (rawCount !== filteredCount) {
            console.warn(`⚠️ Node "${node}" discrepancy: ${rawCount} in raw data, but ${filteredCount} after filtering`);
          }
        });
      }
    }
  }, [processedData, guestData]);
  
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
            color="primary"
            sx={{
              '& .MuiBadge-badge': {
                backgroundColor: theme.palette.mode === 'dark' 
                  ? 'rgba(255, 255, 255, 0.4)' 
                  : 'rgba(0, 0, 0, 0.15)',
                color: theme.palette.mode === 'dark' 
                  ? '#fff' 
                  : 'rgba(0, 0, 0, 0.8)',
                fontWeight: 500,
                fontSize: '0.65rem'
              }
            }}
            invisible={hiddenColumnsCount === 0}
          >
            <ViewColumnIcon />
          </Badge>
        </IconButton>
      </Tooltip>,
      portalRoot
    );
  };
  
  // Listen for showOnlyRunning changes from UserSettingsContext
  useEffect(() => {
    const handleShowOnlyRunningChange = (event) => {
      if (event.detail && event.detail.showOnlyRunning !== undefined) {
        // When showOnlyRunning is true, set showStopped to false (show only running)
        // When showOnlyRunning is false, set showStopped to null (show all)
        setShowStopped(event.detail.showOnlyRunning ? false : null);
        
        console.log('Status filter updated from settings:', 
          event.detail.showOnlyRunning ? 'showing only running' : 'showing all');
      }
    };
    
    window.addEventListener('showOnlyRunningChange', handleShowOnlyRunningChange);
    
    // Initial check if showOnlyRunning is already set
    const showOnlyRunning = localStorage.getItem('app_show_only_running') === 'true';
    if (showOnlyRunning) {
      setShowStopped(false);
    }
    
    return () => {
      window.removeEventListener('showOnlyRunningChange', handleShowOnlyRunningChange);
    };
  }, [setShowStopped]);
  
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
        resetFilters={handleResetFilters}
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
        sharedGuestIdMap={sharedGuestIdMap}
        updateRoleColumnVisibility={updateRoleColumnVisibility}
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