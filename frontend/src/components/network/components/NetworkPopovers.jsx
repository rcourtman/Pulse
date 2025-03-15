import React from 'react';
import {
  Popover,
  Paper,
  Box,
  Typography,
  IconButton,
  Button,
  Slider
} from '@mui/material';
import FilterAltOffIcon from '@mui/icons-material/FilterAltOff';
import { useSearchContext } from '../../../context/SearchContext';

const NetworkPopovers = ({
  // Filter popover props
  filterAnchorEl,
  openFilters,
  handleCloseFilterPopover,
  filters,
  updateFilter,
  handleSliderDragStart,
  handleSliderDragEnd,
  resetFilters,
  
  // Formatters
  formatPercentage,
  formatNetworkRateForFilter
}) => {
  // Get clearSearchTerms from context to ensure search terms are cleared when filters are reset
  const { clearSearchTerms } = useSearchContext();
  
  // Handle resetting all filters including search terms
  const handleResetAllFilters = () => {
    resetFilters();
    clearSearchTerms();
    handleCloseFilterPopover();
  };
  
  return (
    <>
      {/* Filter popover */}
      <Popover
        id="filter-menu"
        anchorEl={filterAnchorEl}
        open={openFilters}
        onClose={handleCloseFilterPopover}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'right',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        PaperProps={{
          elevation: 3,
          sx: { 
            borderRadius: 2,
            overflow: 'hidden'
          }
        }}
      >
        <Paper sx={{ width: 350, p: 0, overflow: 'hidden' }}>
          <Box sx={{ p: 2, borderBottom: '1px solid', borderColor: 'divider' }}>
            <Typography variant="subtitle2" gutterBottom>
              Resource Thresholds
            </Typography>
            <Typography variant="caption" color="text.secondary">
              Filter systems by resource usage thresholds.
            </Typography>
          </Box>
          
          <Box sx={{ p: 2 }}>
            {/* CPU filter */}
            <Box sx={{ mb: 3 }}>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
                <Typography variant="body2">CPU Usage</Typography>
                <Typography variant="caption" color="text.secondary">
                  {formatPercentage(filters.cpu)}
                </Typography>
              </Box>
              <Slider
                id="cpu-filter-slider"
                value={filters.cpu}
                onChange={(e, newValue) => updateFilter('cpu', newValue)}
                onMouseDown={() => handleSliderDragStart('cpu')}
                onMouseUp={handleSliderDragEnd}
                min={0}
                max={100}
                step={1}
                valueLabelDisplay="auto"
                valueLabelFormat={formatPercentage}
                aria-labelledby="cpu-usage-slider"
              />
            </Box>
            
            {/* Memory filter */}
            <Box sx={{ mb: 3 }}>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
                <Typography variant="body2">Memory Usage</Typography>
                <Typography variant="caption" color="text.secondary">
                  {formatPercentage(filters.memory)}
                </Typography>
              </Box>
              <Slider
                id="memory-filter-slider"
                value={filters.memory}
                onChange={(e, newValue) => updateFilter('memory', newValue)}
                onMouseDown={() => handleSliderDragStart('memory')}
                onMouseUp={handleSliderDragEnd}
                min={0}
                max={100}
                step={1}
                valueLabelDisplay="auto"
                valueLabelFormat={formatPercentage}
                aria-labelledby="memory-usage-slider"
              />
            </Box>
            
            {/* Disk filter */}
            <Box sx={{ mb: 3 }}>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
                <Typography variant="body2">Disk Usage</Typography>
                <Typography variant="caption" color="text.secondary">
                  {formatPercentage(filters.disk)}
                </Typography>
              </Box>
              <Slider
                id="disk-filter-slider"
                value={filters.disk}
                onChange={(e, newValue) => updateFilter('disk', newValue)}
                onMouseDown={() => handleSliderDragStart('disk')}
                onMouseUp={handleSliderDragEnd}
                min={0}
                max={100}
                step={1}
                valueLabelDisplay="auto"
                valueLabelFormat={formatPercentage}
                aria-labelledby="disk-usage-slider"
              />
            </Box>
            
            {/* Download filter */}
            <Box sx={{ mb: 3 }}>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
                <Typography variant="body2">Download Rate</Typography>
                <Typography variant="caption" color="text.secondary">
                  {formatNetworkRateForFilter(filters.download)}
                </Typography>
              </Box>
              <Slider
                id="download-filter-slider"
                value={filters.download}
                onChange={(e, newValue) => updateFilter('download', newValue)}
                onMouseDown={() => handleSliderDragStart('download')}
                onMouseUp={handleSliderDragEnd}
                min={0}
                max={100000}
                step={1}
                valueLabelDisplay="auto"
                valueLabelFormat={formatNetworkRateForFilter}
                aria-labelledby="download-rate-slider"
              />
            </Box>
            
            {/* Upload filter */}
            <Box sx={{ mb: 3 }}>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
                <Typography variant="body2">Upload Rate</Typography>
                <Typography variant="caption" color="text.secondary">
                  {formatNetworkRateForFilter(filters.upload)}
                </Typography>
              </Box>
              <Slider
                id="upload-filter-slider"
                value={filters.upload}
                onChange={(e, newValue) => updateFilter('upload', newValue)}
                onMouseDown={() => handleSliderDragStart('upload')}
                onMouseUp={handleSliderDragEnd}
                min={0}
                max={100000}
                step={1}
                valueLabelDisplay="auto"
                valueLabelFormat={formatNetworkRateForFilter}
                aria-labelledby="upload-rate-slider"
              />
            </Box>
            
            {/* Reset button */}
            <Button
              variant="outlined"
              startIcon={<FilterAltOffIcon />}
              onClick={handleResetAllFilters}
              fullWidth
              sx={{ mt: 1 }}
            >
              Reset All Filters
            </Button>
          </Box>
        </Paper>
      </Popover>
    </>
  );
};

export default NetworkPopovers; 