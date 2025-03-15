import React from 'react';
import {
  Box
} from '@mui/material';

const NetworkHeader = ({
  openFilters,
  filters,
  handleFilterButtonClick,
  filterButtonRef
}) => {
  return (
    <Box sx={{ 
      display: 'flex', 
      justifyContent: 'space-between', 
      alignItems: 'center',
      mb: 1,
      mt: 1,
      px: 1
    }}>
      {/* Left side - Intentionally empty, removed Dashboard title */}
      <Box />
      
      {/* Right side - Actions - Column visibility button has been moved to the main app header */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        {/* Additional buttons can be added here if needed */}
      </Box>
    </Box>
  );
};

export default NetworkHeader; 