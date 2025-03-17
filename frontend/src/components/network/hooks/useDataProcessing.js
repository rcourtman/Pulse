import { useMemo, useCallback } from 'react';
import { getSortedAndFilteredData, getNodeFilteredGuests as nodeFilteredGuestsUtil, getNodeName as getNodeNameUtil, extractNumericId as extractNumericIdUtil } from '../../../utils/networkUtils';
import { useSearchContext } from '../../../context/SearchContext';

const useDataProcessing = ({
  guestData,
  nodeData,
  sortConfig,
  filters,
  showStopped,
  selectedNode,
  guestTypeFilter,
  metricsData
}) => {
  // Get search state from context
  const { searchTerm, activeSearchTerms } = useSearchContext();
  
  // Helper function to extract numeric ID from strings like "node-1-ct-105"
  const extractNumericId = useCallback((fullId) => {
    return extractNumericIdUtil(fullId);
  }, []);
  
  // Helper function to get the node name from the node ID
  const getNodeName = useCallback((nodeId) => {
    return getNodeNameUtil(nodeId, nodeData);
  }, [nodeData]);
  
  // Filter guests based on selected node
  const getNodeFilteredGuests = useCallback((guests) => {
    const result = nodeFilteredGuestsUtil(guests, selectedNode);
    
    console.log(`Node filtering for "${selectedNode}":`);
    console.log(`- Before: ${guests?.length || 0} guests`);
    console.log(`- After: ${result?.length || 0} guests`);
    
    if (result?.length > 0) {
      // Log node assignments in the filtered result
      const nodeAssignments = {};
      result.forEach(guest => {
        const nodeId = guest.node;
        if (!nodeAssignments[nodeId]) {
          nodeAssignments[nodeId] = [];
        }
        nodeAssignments[nodeId].push(`${guest.id} (${guest.name})`);
      });
      
      console.log('FILTERED GUESTS BY NODE:');
      Object.keys(nodeAssignments).sort().forEach(nodeId => {
        console.log(`Node "${nodeId}": ${nodeAssignments[nodeId].length} guests`);
        console.log(`  Guests: ${nodeAssignments[nodeId].join(', ')}`);
      });
    }
    
    return result;
  }, [selectedNode]);
  
  // Get sorted and filtered data
  const processedData = useMemo(() => {
    // Debug logging
    console.log('useDataProcessing - Processing data:');
    console.log('- guestData:', guestData?.length || 0, 'guests');
    console.log('- selectedNode:', selectedNode);
    console.log('- sortConfig:', sortConfig);
    
    // Check for duplicate guest IDs
    if (guestData && guestData.length > 0) {
      const guestIds = new Set();
      const duplicates = [];
      
      guestData.forEach(guest => {
        if (guestIds.has(guest.id)) {
          duplicates.push(guest.id);
        } else {
          guestIds.add(guest.id);
        }
      });
      
      if (duplicates.length > 0) {
        console.warn('⚠️ Found duplicate guest IDs:', duplicates);
      }
      
      // Check for guests with unexpected node values
      const nodeAssignments = {};
      guestData.forEach(guest => {
        const nodeId = guest.node;
        if (!nodeAssignments[nodeId]) {
          nodeAssignments[nodeId] = [];
        }
        nodeAssignments[nodeId].push(`${guest.id}`);
      });
      
      console.log('🔍 Current guest node assignments:');
      Object.keys(nodeAssignments).sort().forEach(nodeId => {
        // Check if this is one of our problem nodes
        const isWatchNode = ['pve-prod-01', 'pve-prod-02'].includes(nodeId);
        const marker = isWatchNode ? '⚠️' : '✅';
        console.log(`${marker} Node "${nodeId}": ${nodeAssignments[nodeId].length} guests`);
        
        // For problem nodes, list all the guests 
        if (isWatchNode) {
          console.log(`  Guests: ${nodeAssignments[nodeId].join(', ')}`);
        }
      });
    }
    
    // First filter by node
    const nodeFilteredData = selectedNode === 'all' 
      ? guestData 
      : getNodeFilteredGuests(guestData);
    
    console.log('- nodeFilteredData:', nodeFilteredData?.length || 0, 'guests after node filtering');
    
    // Then apply all other filters and sorting
    const result = getSortedAndFilteredData(
      nodeFilteredData,
      sortConfig,
      filters,
      showStopped,
      activeSearchTerms,
      searchTerm,
      metricsData,
      guestTypeFilter,
      nodeData
    );
    
    // Final detailed logging to debug the issue
    if (result && result.length > 0) {
      console.log(`🔍 FINAL: ${result.length} guests will be displayed`);
      const finalNodeAssignments = {};
      result.forEach(guest => {
        const nodeId = guest.node || 'unknown';
        if (!finalNodeAssignments[nodeId]) {
          finalNodeAssignments[nodeId] = [];
        }
        finalNodeAssignments[nodeId].push(guest.id);
      });
      
      console.log('🔍 FINAL node distribution:');
      Object.keys(finalNodeAssignments).sort().forEach(nodeId => {
        console.log(`  - ${nodeId}: ${finalNodeAssignments[nodeId].length} guests`);
        console.log(`    Guest IDs: ${finalNodeAssignments[nodeId].join(', ')}`);
      });
    }
    
    console.log('- Final filtered data:', result?.length || 0, 'guests');
    return result;
  }, [
    guestData, 
    sortConfig, 
    filters, 
    showStopped, 
    activeSearchTerms, 
    searchTerm, 
    selectedNode, 
    getNodeFilteredGuests,
    guestTypeFilter,
    nodeData,
    metricsData
  ]);

  // Format percentage for display
  const formatPercentage = useCallback((value) => {
    return `${value}%`;
  }, []);
  
  // Format network rate for filter display
  const formatNetworkRateForFilter = useCallback((value) => {
    if (value === 0) return '0 KB/s';
    if (value <= 10) return `${value * 10} KB/s`;
    if (value <= 50) return `${(value - 10) * 20 + 100} KB/s`;
    if (value <= 80) return `${(value - 50) * 50 + 900} KB/s`;
    return `${(value - 80) * 500 + 2400} KB/s`;
  }, []);

  return {
    extractNumericId,
    getNodeName,
    getNodeFilteredGuests,
    processedData,
    formatPercentage,
    formatNetworkRateForFilter
  };
};

export default useDataProcessing; 