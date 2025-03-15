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
    return nodeFilteredGuestsUtil(guests, selectedNode);
  }, [selectedNode]);
  
  // Get sorted and filtered data
  const processedData = useMemo(() => {
    // Debug logging
    console.log('useDataProcessing - Processing data:');
    console.log('- guestData:', guestData?.length || 0, 'guests');
    console.log('- selectedNode:', selectedNode);
    console.log('- sortConfig:', sortConfig);
    
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