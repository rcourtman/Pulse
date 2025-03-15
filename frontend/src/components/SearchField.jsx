import React, { useRef, useEffect, useState } from 'react';
import { 
  Box, 
  InputBase, 
  IconButton, 
  Tooltip, 
  Chip,
  alpha,
  Paper,
  Badge,
  Popover,
  Typography,
  Divider
} from '@mui/material';
import SearchIcon from '@mui/icons-material/Search';
import ClearIcon from '@mui/icons-material/Clear';
import FilterListIcon from '@mui/icons-material/FilterList';
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined';
import AddIcon from '@mui/icons-material/Add';
import { useSearchContext } from '../context/SearchContext';
import { useTheme } from '@mui/material/styles';

const SearchField = () => {
  const theme = useTheme();
  const { 
    searchTerm, 
    setSearchTerm, 
    activeSearchTerms, 
    addSearchTerm, 
    removeSearchTerm,
    clearSearchTerms,
    isSearching,
    setIsSearching
  } = useSearchContext();
  
  const searchInputRef = useRef(null);
  
  // State for filter bubble popover
  const [filterAnchorEl, setFilterAnchorEl] = useState(null);
  const openFilters = Boolean(filterAnchorEl);
  
  // State to track if we added a search prefix that needs cursor positioning
  const [addedPrefix, setAddedPrefix] = useState(false);
  
  // Position cursor after a prefix is added
  useEffect(() => {
    if (addedPrefix && searchInputRef.current) {
      // Small delay to ensure input is updated
      setTimeout(() => {
        searchInputRef.current.focus();
        searchInputRef.current.setSelectionRange(searchTerm.length, searchTerm.length);
        setAddedPrefix(false);
      }, 50);
    }
  }, [addedPrefix, searchTerm]);
  
  // Handle opening filter bubble
  const handleFilterClick = (event) => {
    setFilterAnchorEl(event.currentTarget);
  };
  
  // Handle closing filter bubble
  const handleFilterClose = () => {
    setFilterAnchorEl(null);
  };

  // Focus the search input when isSearching becomes true
  useEffect(() => {
    if (isSearching && searchInputRef.current) {
      searchInputRef.current.focus();
    }
  }, [isSearching]);

  // Handle keyboard shortcuts in search field
  useEffect(() => {
    const handleKeyDown = (e) => {
      // Only handle events when the search input is focused
      if (e.target === searchInputRef.current) {
        console.log('SearchField keydown event:', e.key, 'Current searchTerm:', searchTerm);
        
        // Handle Escape key to clear all filters
        if (e.key === 'Escape') {
          e.preventDefault();
          setSearchTerm('');
          clearSearchTerms();
          setIsSearching(false);
          searchInputRef.current.blur();
        }
        
        // Handle Backspace key to remove last filter when input is empty
        if (e.key === 'Backspace' && !searchTerm && activeSearchTerms.length > 0) {
          e.preventDefault();
          // Remove the last filter that was added
          const lastTerm = activeSearchTerms[activeSearchTerms.length - 1];
          removeSearchTerm(lastTerm);
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [setSearchTerm, clearSearchTerms, setIsSearching, searchTerm, activeSearchTerms, removeSearchTerm]);

  // Handle search submission
  const handleSearchSubmit = (e) => {
    e.preventDefault();
    if (searchTerm.trim()) {
      addSearchTerm(searchTerm.trim());
      setSearchTerm('');
    }
  };

  // Handle example filter click
  const handleExampleClick = (filterTerm) => {
    addSearchTerm(filterTerm);
    setFilterAnchorEl(null); // Close the popover after adding
  };

  // Categorize search terms into different types
  const categorizeSearchTerms = () => {
    const categories = {
      status: [],
      type: [],
      node: [],
      other: []
    };

    activeSearchTerms.forEach(term => {
      if (term.startsWith('status:')) {
        categories.status.push(term);
      } else if (term.startsWith('type:')) {
        categories.type.push(term);
      } else if (term.startsWith('node:')) {
        categories.node.push(term);
      } else {
        categories.other.push(term);
      }
    });

    return categories;
  };

  const filteredCategories = categorizeSearchTerms();
  const hasActiveFilters = activeSearchTerms.length > 0;

  // Custom chip style for example filters
  const exampleChipStyle = {
    fontSize: '0.75rem',
    height: 24,
    mr: 0.75,
    mb: 0.75,
    borderStyle: 'dashed',
    borderWidth: '1px',
    cursor: 'pointer',
    '&:hover': {
      opacity: 0.8,
      boxShadow: `0 0 0 1px ${alpha(theme.palette.primary.main, 0.3)}`
    }
  };

  return (
    <Box sx={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
      {/* Search input */}
      <Box
        component="form"
        onSubmit={handleSearchSubmit}
        sx={{
          display: 'flex',
          alignItems: 'center',
          borderRadius: 2,
          bgcolor: alpha(theme.palette.common.white, 0.15),
          '&:hover': {
            bgcolor: alpha(theme.palette.common.white, 0.25),
          },
          width: { xs: 180, sm: 250 },
        }}
      >
        <Tooltip title="Search (/ or Ctrl+F)">
          <Box 
            sx={{ 
              display: 'flex', 
              alignItems: 'center', 
              justifyContent: 'center',
              p: 1
            }}
          >
            <SearchIcon sx={{ color: 'inherit', fontSize: '1.2rem' }} />
          </Box>
        </Tooltip>
        
        <InputBase
          placeholder="Search..."
          value={searchTerm}
          onChange={(e) => {
            // Log the current input value
            console.log('Search input onChange:', e.target.value, 'Previous value:', searchTerm);
            
            // Directly set the search term to the current input value
            // This should ensure the value is properly updated
            setSearchTerm(e.target.value);
          }}
          onFocus={() => setIsSearching(true)}
          onBlur={() => {
            // Only set isSearching to false if there's no search term
            if (!searchTerm) {
              setIsSearching(false);
            }
          }}
          inputRef={searchInputRef}
          sx={{
            color: 'inherit',
            flex: 1,
            '& .MuiInputBase-input': {
              p: '4px 0',
              width: '100%',
              fontSize: '0.875rem',
            },
          }}
          endAdornment={
            searchTerm ? (
              <IconButton
                size="small"
                aria-label="clear search"
                onClick={() => {
                  setSearchTerm('');
                  // Focus the input after clearing
                  searchInputRef.current?.focus();
                }}
                sx={{ 
                  p: 0.5, 
                  color: 'inherit',
                  opacity: 0.7,
                  '&:hover': {
                    opacity: 1,
                  }
                }}
              >
                <ClearIcon fontSize="small" />
              </IconButton>
            ) : null
          }
        />
      </Box>

      {/* Filter bubble button - always shown */}
      <Tooltip title={hasActiveFilters ? "Active search filters" : "Search tips & filters"}>
        <Badge
          badgeContent={hasActiveFilters ? activeSearchTerms.length : 0}
          color="primary"
          sx={{ ml: 1 }}
          invisible={!hasActiveFilters}
        >
          <IconButton
            size="small"
            aria-label="show active filters"
            onClick={handleFilterClick}
            color="inherit"
            sx={{ 
              '&:hover': { 
                bgcolor: alpha(theme.palette.common.white, 0.15) 
              }
            }}
          >
            <FilterListIcon fontSize="small" />
          </IconButton>
        </Badge>
      </Tooltip>

      {/* Filter popover with search terms */}
      <Popover
        open={openFilters}
        anchorEl={filterAnchorEl}
        onClose={handleFilterClose}
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
            mt: 0.5,
            width: 320,
            overflow: 'hidden'
          }
        }}
      >
        <Paper sx={{ maxHeight: 400, overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
          {/* Header */}
          <Box sx={{ p: 2, bgcolor: 'background.default', borderBottom: '1px solid', borderColor: 'divider' }}>
            <Typography variant="subtitle2" gutterBottom>
              {hasActiveFilters ? "Active Search Filters" : "Search Help"}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {hasActiveFilters ? "These filters are currently applied to your search results." : "Click on any filter example below to apply it."}
            </Typography>
          </Box>
          
          {/* Filters section - only show if there are active filters */}
          {hasActiveFilters && (
            <Box sx={{ p: 1.5, overflow: 'auto', flex: 1 }}>
              {/* Status filters */}
              {filteredCategories.status.length > 0 && (
                <Box sx={{ mb: 1.5 }}>
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                    Status Filters:
                  </Typography>
                  <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                    {filteredCategories.status.map((term) => (
                      <Chip
                        key={term}
                        label={term}
                        size="small"
                        color="info"
                        onDelete={() => removeSearchTerm(term)}
                        sx={{ 
                          fontSize: '0.75rem',
                          height: 24
                        }}
                      />
                    ))}
                  </Box>
                </Box>
              )}

              {/* Type filters */}
              {filteredCategories.type.length > 0 && (
                <Box sx={{ mb: 1.5 }}>
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                    System Type Filters:
                  </Typography>
                  <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                    {filteredCategories.type.map((term) => (
                      <Chip
                        key={term}
                        label={term}
                        size="small"
                        color="success"
                        onDelete={() => removeSearchTerm(term)}
                        sx={{ 
                          fontSize: '0.75rem',
                          height: 24
                        }}
                      />
                    ))}
                  </Box>
                </Box>
              )}

              {/* Node filters */}
              {filteredCategories.node.length > 0 && (
                <Box sx={{ mb: 1.5 }}>
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                    Node Filters:
                  </Typography>
                  <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                    {filteredCategories.node.map((term) => (
                      <Chip
                        key={term}
                        label={term}
                        size="small"
                        color="warning"
                        onDelete={() => removeSearchTerm(term)}
                        sx={{ 
                          fontSize: '0.75rem',
                          height: 24
                        }}
                      />
                    ))}
                  </Box>
                </Box>
              )}

              {/* Text search terms */}
              {filteredCategories.other.length > 0 && (
                <Box sx={{ mb: 1.5 }}>
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                    Text Search:
                  </Typography>
                  <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                    {filteredCategories.other.map((term) => (
                      <Chip
                        key={term}
                        label={term}
                        size="small"
                        color="default"
                        onDelete={() => removeSearchTerm(term)}
                        sx={{ 
                          fontSize: '0.75rem',
                          height: 24
                        }}
                      />
                    ))}
                  </Box>
                </Box>
              )}

              {/* Clear all button */}
              {activeSearchTerms.length > 1 && (
                <Box sx={{ display: 'flex', justifyContent: 'flex-end', mt: 1 }}>
                  <Chip
                    label="Clear all filters"
                    size="small"
                    color="primary"
                    variant="outlined"
                    onClick={clearSearchTerms}
                    sx={{ 
                      fontSize: '0.75rem',
                      height: 24
                    }}
                  />
                </Box>
              )}
            </Box>
          )}
          
          {/* Search tips - always visible */}
          {!hasActiveFilters ? (
            <Box sx={{ p: 1.5, overflow: 'auto', flex: 1 }}>
              <Typography variant="caption" color="text.secondary" component="div" sx={{ mb: 2 }}>
                Use filters to quickly find the exact systems you're looking for. Combine multiple filters for more specific results.
              </Typography>
              
              {/* Filter by node examples */}
              <Box>
                <Typography variant="caption" color="text.secondary" sx={{ mb: 0.5, display: 'block' }}>
                  Filter by node location:
                </Typography>
                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                  <Chip 
                    label="node:" 
                    size="small" 
                    variant="outlined"
                    color="warning" 
                    icon={<AddIcon fontSize="small" />}
                    onClick={() => {
                      setSearchTerm("node:");
                      setAddedPrefix(true);
                      setFilterAnchorEl(null);
                    }}
                    sx={exampleChipStyle}
                  />
                </Box>
              </Box>
              
              {/* Filter by status examples */}
              <Box sx={{ mt: 1 }}>
                <Typography variant="caption" color="text.secondary" sx={{ mb: 0.5, display: 'block' }}>
                  Filter by system status:
                </Typography>
                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                  <Chip 
                    label="status:running" 
                    size="small" 
                    variant="outlined"
                    color="info" 
                    icon={<AddIcon fontSize="small" />}
                    onClick={() => handleExampleClick("status:running")}
                    sx={exampleChipStyle}
                  />
                  <Chip 
                    label="status:stopped" 
                    size="small" 
                    variant="outlined"
                    color="info" 
                    icon={<AddIcon fontSize="small" />}
                    onClick={() => handleExampleClick("status:stopped")}
                    sx={exampleChipStyle}
                  />
                  <Chip 
                    label="status:paused" 
                    size="small" 
                    variant="outlined"
                    color="info" 
                    icon={<AddIcon fontSize="small" />}
                    onClick={() => handleExampleClick("status:paused")}
                    sx={exampleChipStyle}
                  />
                </Box>
              </Box>
              
              {/* Filter by type examples */}
              <Box sx={{ mt: 1 }}>
                <Typography variant="caption" color="text.secondary" sx={{ mb: 0.5, display: 'block' }}>
                  Filter by system type:
                </Typography>
                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                  <Chip 
                    label="type:vm" 
                    size="small" 
                    variant="outlined"
                    color="success" 
                    icon={<AddIcon fontSize="small" />}
                    onClick={() => handleExampleClick("type:vm")}
                    sx={exampleChipStyle}
                  />
                  <Chip 
                    label="type:ct" 
                    size="small" 
                    variant="outlined"
                    color="success" 
                    icon={<AddIcon fontSize="small" />}
                    onClick={() => handleExampleClick("type:ct")}
                    sx={exampleChipStyle}
                  />
                </Box>
              </Box>
              
              {/* Filter by resources examples */}
              <Box sx={{ mt: 1 }}>
                <Typography variant="caption" color="text.secondary" sx={{ mb: 0.5, display: 'block' }}>
                  Filter by resource usage thresholds:
                </Typography>
                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                  <Chip 
                    label="cpu>50" 
                    size="small" 
                    variant="outlined"
                    color="info" 
                    icon={<AddIcon fontSize="small" />}
                    onClick={() => handleExampleClick("cpu>50")}
                    sx={exampleChipStyle}
                  />
                  <Chip 
                    label="memory>80" 
                    size="small" 
                    variant="outlined"
                    color="info" 
                    icon={<AddIcon fontSize="small" />}
                    onClick={() => handleExampleClick("memory>80")}
                    sx={exampleChipStyle}
                  />
                  <Chip 
                    label="disk>90" 
                    size="small" 
                    variant="outlined"
                    color="info" 
                    icon={<AddIcon fontSize="small" />}
                    onClick={() => handleExampleClick("disk>90")}
                    sx={exampleChipStyle}
                  />
                  <Chip 
                    label="network>100" 
                    size="small" 
                    variant="outlined"
                    color="info" 
                    icon={<AddIcon fontSize="small" />}
                    onClick={() => handleExampleClick("network>100")}
                    sx={exampleChipStyle}
                  />
                </Box>
              </Box>

              {/* Text search examples */}
              <Box sx={{ mt: 1 }}>
                <Typography variant="caption" color="text.secondary" sx={{ mb: 0.5, display: 'block' }}>
                  Free text search:
                </Typography>
                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                  <Chip 
                    label="web" 
                    size="small" 
                    variant="outlined"
                    color="default" 
                    icon={<AddIcon fontSize="small" />}
                    onClick={() => handleExampleClick("web")}
                    sx={exampleChipStyle}
                  />
                  <Chip 
                    label="database" 
                    size="small" 
                    variant="outlined"
                    color="default" 
                    icon={<AddIcon fontSize="small" />}
                    onClick={() => handleExampleClick("database")}
                    sx={exampleChipStyle}
                  />
                </Box>
              </Box>
              
              <Box sx={{ mt: 2, pt: 1, borderTop: 1, borderColor: 'divider' }}>
                <Typography variant="caption" color="text.secondary">
                  Tip: You can combine multiple filters (e.g., "type:vm status:running cpu{'>'}50").
                </Typography>
              </Box>
            </Box>
          ) : (
            <Divider />
          )}
          
          {/* Power user tip at the bottom */}
          <Box sx={{ p: 1.5, bgcolor: alpha(theme.palette.primary.main, 0.05) }}>
            <Box sx={{ display: 'flex', alignItems: 'center', mb: 0.5 }}>
              <InfoOutlinedIcon color="primary" fontSize="small" sx={{ mr: 0.5 }} />
              <Typography variant="caption" fontWeight="medium" color="primary.main">
                Keyboard Shortcuts
              </Typography>
            </Box>
            <Typography variant="caption" color="text.secondary" component="div" sx={{ mt: 0.5 }}>
              • Press <strong>/</strong> or <strong>Ctrl+F</strong> from any screen to focus search<br />
              • <strong>Backspace</strong> removes the last filter when input is empty<br />
              • <strong>ESC</strong> clears all active filters and closes search<br />
              • <strong>Enter</strong> adds your current search term as a filter
            </Typography>
          </Box>
        </Paper>
      </Popover>
    </Box>
  );
};

export default SearchField; 