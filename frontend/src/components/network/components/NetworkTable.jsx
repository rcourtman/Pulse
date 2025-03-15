import React, { useState, useEffect } from 'react';
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
            />
          </Table>
        </TableContainer>
      </CardContent>
    </Card>
  );
};

export default NetworkTable; 