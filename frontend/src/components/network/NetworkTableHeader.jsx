import React, { useMemo, useState, useEffect, useCallback } from 'react';
import {
  TableHead,
  TableRow,
  TableCell,
  TableSortLabel,
  Box,
  IconButton,
  Menu,
  MenuItem,
  Tooltip,
  Badge,
  Divider,
  Typography,
  Button,
  useTheme,
  alpha,
  ListItemIcon,
  ListItemText,
  Select,
  FormControl,
  Popover,
  Fade,
  Grow,
  Slider
} from '@mui/material';
import ViewColumnIcon from '@mui/icons-material/ViewColumn';
import VisibilityOffIcon from '@mui/icons-material/VisibilityOff';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import TuneIcon from '@mui/icons-material/Tune';
import CheckIcon from '@mui/icons-material/Check';
import DragIndicatorIcon from '@mui/icons-material/DragIndicator';
import CircleIcon from '@mui/icons-material/Circle';
import CircleOutlinedIcon from '@mui/icons-material/CircleOutlined';
import AdjustIcon from '@mui/icons-material/Adjust';
import TripOriginIcon from '@mui/icons-material/TripOrigin';
import ViewListIcon from '@mui/icons-material/ViewList';
import DnsIcon from '@mui/icons-material/Dns';
import ComputerIcon from '@mui/icons-material/Computer';
import KeyboardArrowDownIcon from '@mui/icons-material/KeyboardArrowDown';
import KeyboardArrowUpIcon from '@mui/icons-material/KeyboardArrowUp';
import FilterListIcon from '@mui/icons-material/FilterList';
import PlayArrowIcon from '@mui/icons-material/PlayArrow';
import StopIcon from '@mui/icons-material/Stop';
import AllInclusiveIcon from '@mui/icons-material/AllInclusive';
import StorageIcon from '@mui/icons-material/Storage';
import SearchIcon from '@mui/icons-material/Search';
import CloseIcon from '@mui/icons-material/Close';
import MoreVertIcon from '@mui/icons-material/MoreVert';
import { DEFAULT_COLUMN_CONFIG } from '../../constants/networkConstants';
import { calculateDynamicColumnWidths } from '../../utils/networkUtils';
import { useSearchContext } from '../../context/SearchContext';
import { useSnackbar } from 'notistack';

// Modify the Resizer component to be a no-op since we're removing manual resizing
const Resizer = () => null;

const NetworkTableHeader = ({
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
  activeFilteredColumns = {},
  showStopped = null,
  guestTypeFilter = 'all',
  setShowStopped,
  showRunningOnly = true,
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
  const { enqueueSnackbar } = useSnackbar();

  // Remove resize state and localStorage logic since we're eliminating manual resizing
  // Instead, we'll rely on intelligent auto-sizing

  // Define column groups for sizing strategy
  const fixedNarrowColumns = ['type', 'id', 'status']; // Very narrow columns
  const fixedWidthColumns = ['download', 'upload', 'uptime']; // Fixed width for network stats
  const autoSizeColumns = ['name', 'node']; // Auto-size to content
  const flexibleEqualColumns = ['cpu', 'memory', 'disk']; // Equal width with progress bars
  
  // Calculate visible columns and their groups
  const getVisibleColumnGroups = useCallback(() => {
    if (!columnOrder || !Array.isArray(columnOrder) || columnOrder.length === 0) {
      return { fixedNarrow: [], fixedWidth: [], autoSize: [], flexibleEqual: [] };
    }
    
    const visibleColumnIds = columnOrder.filter(id => columnVisibility[id]?.visible);
    
    return {
      fixedNarrow: visibleColumnIds.filter(id => fixedNarrowColumns.includes(id)),
      fixedWidth: visibleColumnIds.filter(id => fixedWidthColumns.includes(id)),
      autoSize: visibleColumnIds.filter(id => autoSizeColumns.includes(id)),
      flexibleEqual: visibleColumnIds.filter(id => flexibleEqualColumns.includes(id))
    };
  }, [columnOrder, columnVisibility]);

  // Intelligent column width calculation
  const getColumnWidth = useCallback((columnId) => {
    // Define fixed widths for specific columns
    const fixedWidths = {
      type: '50px',     // Just "VM" or "CT"
      id: '60px',       // Just numeric IDs
      status: '50px',   // Just the status circle
      download: '90px', // Network rates
      upload: '90px',   // Network rates
      uptime: '90px'    // Time display
    };
    
    // If column has a fixed width, return it
    if (fixedWidths[columnId]) {
      return fixedWidths[columnId];
    }
    
    // Get all visible column groups
    const columnGroups = getVisibleColumnGroups();
    
    // For auto-sized columns, use different strategies for name vs node
    if (autoSizeColumns.includes(columnId)) {
      // Use max-content for node to fit content more closely
      if (columnId === 'node') {
        return 'max-content';
      }
      // Use auto for name which can be longer
      return 'auto';
    }
    
    // For flexible equal columns (cpu, memory, disk), calculate percentage 
    if (flexibleEqualColumns.includes(columnId)) {
      // Calculate how many flexible columns are visible
      const visibleFlexColumns = columnGroups.flexibleEqual?.length || 0;
      
      if (visibleFlexColumns > 0) {
        // Calculate total width taken by fixed columns
        // (This is an approximation since auto columns are unknown)
        const approximateFixedWidth = 
          ((columnGroups.fixedNarrow?.length || 0) * 60) + 
          ((columnGroups.fixedWidth?.length || 0) * 90) + 
          ((columnGroups.autoSize?.length || 0) * 120); // Rough estimate for auto columns
        
        // Approximate remaining percentage for flexible columns
        // Assuming table width is roughly 1000px (this is just a heuristic)
        const remainingPercentage = Math.max(10, (1000 - approximateFixedWidth) / 10); // in %
        
        // Divide equally among visible flexible columns
        return `${Math.floor(remainingPercentage / visibleFlexColumns)}%`;
      }
      
      // Fallback if something goes wrong
      return '20%';
    }
    
    // Default fallback
    return 'auto';
  }, [getVisibleColumnGroups]);

  // Helper to check column type
  const isFlexibleColumn = (columnId) => {
    return flexibleEqualColumns.includes(columnId);
  };

  const [nodeMenuAnchorEl, setNodeMenuAnchorEl] = React.useState(null);
  const openNodeMenu = Boolean(nodeMenuAnchorEl);
  
  const [statusMenuAnchorEl, setStatusMenuAnchorEl] = React.useState(null);
  const openStatusMenu = Boolean(statusMenuAnchorEl);
  
  const [typeMenuAnchorEl, setTypeMenuAnchorEl] = React.useState(null);
  const openTypeMenu = Boolean(typeMenuAnchorEl);
  
  // Get search state from context
  const { activeSearchTerms, addSearchTerm, removeSearchTerm } = useSearchContext();
  
  const handleNodeMenuClose = () => {
    setNodeMenuAnchorEl(null);
  };
  
  const handleNodeSelect = (nodeId) => {
    // Update dropdown filter state
    handleNodeChange({ target: { value: nodeId } });
    
    // Also add the corresponding search term if not "all"
    if (nodeId !== 'all') {
      // Find the node name for better readability in the search term
      const selectedNode = availableNodes.find(node => node.id === nodeId);
      const nodeName = selectedNode ? selectedNode.name : nodeId;
      
      // Find and remove any existing node: filters
      const existingNodeTerms = activeSearchTerms.filter(term => 
        term.toLowerCase().startsWith('node:')
      );
      existingNodeTerms.forEach(term => {
        removeSearchTerm(term);
      });
      
      // Add the new node filter as a search term
      addSearchTerm(`node:${nodeName}`);
    } else {
      // Remove any node: filters when "all" is selected
      const existingNodeTerms = activeSearchTerms.filter(term => 
        term.toLowerCase().startsWith('node:')
      );
      existingNodeTerms.forEach(term => {
        removeSearchTerm(term);
      });
    }
    
    handleNodeMenuClose();
  };
  
  const handleStatusMenuClose = () => {
    setStatusMenuAnchorEl(null);
  };
  
  const handleStatusSelect = (status) => {
    // Update dropdown filter state
    if (status === 'all') {
      handleStatusChange(null);
    } else if (status === 'running') {
      handleStatusChange(false);
    } else if (status === 'stopped') {
      handleStatusChange(true);
    }
    
    // Also add the corresponding search term
    // Find and remove any existing status: filters
    const existingStatusTerms = activeSearchTerms.filter(term => 
      term.toLowerCase().startsWith('status:')
    );
    existingStatusTerms.forEach(term => {
      removeSearchTerm(term);
    });
    
    // Add new status filter as a search term (except for "all" which means no filter)
    if (status !== 'all') {
      addSearchTerm(`status:${status}`);
    }
    
    handleStatusMenuClose();
  };
  
  const handleTypeMenuClose = () => {
    setTypeMenuAnchorEl(null);
  };
  
  const handleTypeSelect = (type) => {
    // Update dropdown filter state
    handleTypeChange(type);
    
    // Also add the corresponding search term
    // Find and remove any existing type: filters
    const existingTypeTerms = activeSearchTerms.filter(term => 
      term.toLowerCase().startsWith('type:')
    );
    existingTypeTerms.forEach(term => {
      removeSearchTerm(term);
    });
    
    // Add new type filter as a search term (except for "all" which means no filter)
    if (type !== 'all') {
      addSearchTerm(`type:${type}`);
    }
    
    handleTypeMenuClose();
  };

  const getSortDirection = (key) => {
    if (sortConfig?.key === key) {
      return sortConfig.direction;
    }
    return undefined;
  };

  const isSortingEnabledForType = guestTypeFilter === 'all';

  const handleColumnClick = (columnId, forcedDirection) => {
    // For normal columns, just requestSort
    console.log(`Requesting sort for column: ${columnId}`, forcedDirection ? `with forced direction: ${forcedDirection}` : '');
    
    // Always pass the current columnId and the forcedDirection if provided
    requestSort(columnId, forcedDirection);
  };

  const visibleColumnCount = Object.values(columnVisibility).filter(col => col.visible).length;
  
  const hasVisibleColumns = visibleColumnCount > 0;

  const columnWidths = useMemo(() => {
    return calculateDynamicColumnWidths(columnVisibility);
  }, [columnVisibility, forceUpdateCounter]);

  const visibleColumns = useMemo(() => {
    if (!columnOrder || !Array.isArray(columnOrder) || columnOrder.length === 0) {
      return [];
    }
    
    const visible = Object.entries(columnVisibility || {})
      .filter(([_, config]) => config?.visible)
      .map(([id, config]) => ({ id, ...config }));
    
    return columnOrder
      .filter(id => columnVisibility?.[id]?.visible)
      .map(id => {
        const column = visible.find(col => col.id === id);
        return column || null;
      })
      .filter(Boolean);
  }, [columnVisibility, columnOrder, forceUpdateCounter]);

  const moveColumnUp = (columnId) => {
    const currentIndex = columnOrder.indexOf(columnId);
    if (currentIndex > 0) {
      const newOrder = [...columnOrder];
      [newOrder[currentIndex - 1], newOrder[currentIndex]] = [newOrder[currentIndex], newOrder[currentIndex - 1]];
      setColumnOrder(newOrder);
    }
  };

  const moveColumnDown = (columnId) => {
    const currentIndex = columnOrder.indexOf(columnId);
    if (currentIndex < columnOrder.length - 1) {
      const newOrder = [...columnOrder];
      [newOrder[currentIndex], newOrder[currentIndex + 1]] = [newOrder[currentIndex + 1], newOrder[currentIndex]];
      setColumnOrder(newOrder);
    }
  };

  const [draggedColumn, setDraggedColumn] = React.useState(null);
  const [dragOverColumn, setDragOverColumn] = React.useState(null);
  const [previewOrder, setPreviewOrder] = React.useState(null);
  const dragHandleRef = React.useRef(null);

  // Add cleanup for drag event classes
  React.useEffect(() => {
    // Cleanup function to ensure we don't leave classes when component unmounts
    return () => {
      // Remove any drag-related classes
      document.querySelectorAll('.drop-highlight-before, .drop-highlight-after')
        .forEach(el => {
          el.classList.remove('drop-highlight-before', 'drop-highlight-after');
        });
      
      // Remove dragging class from body
      document.body.classList.remove('column-dragging');
    };
  }, []);

  const handleDragStart = (e, columnId) => {
    console.log('Drag start:', columnId);
    setDraggedColumn(columnId);
    setPreviewOrder([...columnOrder]);
    
    // Create a custom drag image
    const draggedElement = e.currentTarget;
    const rect = draggedElement.getBoundingClientRect();
    
    const ghostElement = draggedElement.cloneNode(true);
    ghostElement.style.width = `${rect.width}px`;
    ghostElement.style.height = `${rect.height}px`;
    ghostElement.style.backgroundColor = alpha(theme.palette.primary.main, 0.2);
    ghostElement.style.boxShadow = `0 4px 8px ${alpha(theme.palette.primary.dark, 0.2)}`;
    ghostElement.style.borderRadius = '4px';
    ghostElement.style.opacity = '0.8';
    ghostElement.style.position = 'absolute';
    ghostElement.style.top = '-1000px';
    ghostElement.style.left = '-1000px';
    ghostElement.style.zIndex = '9999';
    ghostElement.style.pointerEvents = 'none';

    document.body.appendChild(ghostElement);
    
    e.dataTransfer.setDragImage(ghostElement, rect.width / 2, rect.height / 2);
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', columnId);
    
    // Add a class to the body to show we're dragging
    document.body.classList.add('column-dragging');
    
    // Store reference to the ghost element to ensure cleanup
    const ghostRef = ghostElement;
    
    // Clean up ghost element immediately after drag image is set
    // Use requestAnimationFrame to ensure it happens after browser processes the drag image
    requestAnimationFrame(() => {
      // Check if the element is still in the DOM before removing
      if (ghostRef && ghostRef.parentNode) {
        ghostRef.parentNode.removeChild(ghostRef);
      }
    });
  };

  const handleDragOver = (e, columnId) => {
    e.preventDefault();
    if (draggedColumn === columnId) return;
    
    setDragOverColumn(columnId);
    
    // Add class to the element for highlight
    const targetCell = e.currentTarget;
    const dropPosition = getDropPosition(draggedColumn, columnId);
    
    // Remove any existing highlight classes first
    targetCell.classList.remove('drop-highlight-before', 'drop-highlight-after');
    
    // Add the appropriate highlight class
    if (dropPosition === 'before') {
      targetCell.classList.add('drop-highlight-before');
    } else {
      targetCell.classList.add('drop-highlight-after');
    }
    
    // Update the preview order if needed
    if (columnOrder && draggedColumn && columnId !== draggedColumn) {
      const fromIndex = columnOrder.indexOf(draggedColumn);
      const toIndex = columnOrder.indexOf(columnId);
      
      if (fromIndex !== -1 && toIndex !== -1) {
        const newOrder = [...columnOrder];
        newOrder.splice(fromIndex, 1);
        newOrder.splice(toIndex, 0, draggedColumn);
        setPreviewOrder(newOrder);
      }
    }
  };

  const handleDrop = (e, columnId) => {
    e.preventDefault();
    
    // Remove highlight classes
    e.currentTarget.classList.remove('drop-highlight-before', 'drop-highlight-after');
    
    if (draggedColumn !== columnId && columnOrder) {
      console.log('Using fallback drop logic');
      const sourceIndex = columnOrder.indexOf(draggedColumn);
      const targetIndex = columnOrder.indexOf(columnId);
      
      if (sourceIndex !== -1 && targetIndex !== -1) {
        const newOrder = [...columnOrder];
        newOrder.splice(sourceIndex, 1);
        newOrder.splice(targetIndex, 0, draggedColumn);
        setColumnOrder(newOrder);
      }
    }
    
    setDraggedColumn(null);
    setDragOverColumn(null);
    setPreviewOrder(null);
  };

  const handleDragEnd = () => {
    // Remove all highlight classes from the table cells
    document.querySelectorAll('.drop-highlight-before, .drop-highlight-after')
      .forEach(el => {
        el.classList.remove('drop-highlight-before', 'drop-highlight-after');
      });
    
    setDraggedColumn(null);
    setDragOverColumn(null);
    setPreviewOrder(null);
    
    // Remove the dragging class
    document.body.classList.remove('column-dragging');
  };

  const getDropPosition = (sourceId, targetId) => {
    if (!columnOrder || !sourceId || !targetId) return 'before';
    
    const sourceIndex = columnOrder.indexOf(sourceId);
    const targetIndex = columnOrder.indexOf(targetId);
    
    if (sourceIndex === -1 || targetIndex === -1) return 'before';
    
    // If source comes before target in the original order, drop after
    // If source comes after target in the original order, drop before
    return sourceIndex < targetIndex ? 'after' : 'before';
  };
  
  const displayOrder = previewOrder || columnOrder;

  const selectedNodeObject = availableNodes.find(node => node.id === selectedNode);
  const selectedNodeName = selectedNodeObject ? selectedNodeObject.name : 'All Nodes';

  const showNotification = (message, variant = 'info') => {
    enqueueSnackbar(message, { 
      variant, 
      autoHideDuration: 3000,
      anchorOrigin: {
        vertical: 'top',
        horizontal: 'center',
      }
    });
  };

  // State for column header hover
  const [hoveredColumn, setHoveredColumn] = useState(null);
  
  // Function to handle column header mouse enter
  const handleColumnMouseEnter = (columnId) => () => {
    setHoveredColumn(columnId);
  };
  
  // Function to handle column header mouse leave
  const handleColumnMouseLeave = () => {
    setHoveredColumn(null);
  };
  
  // Function to hide column
  const hideColumn = (columnId, e) => {
    if (e) e.stopPropagation();
    toggleColumnVisibility(columnId);
    setHoveredColumn(null);
  };
  
  // Handle opening resource-specific filter
  const handleResourceFilterClick = (event, resourceType) => {
    event.stopPropagation();
    
    // Direct reference to the main filter button's click handler
    handleFilterButtonClick(event);
    
    // Update filter to match the resource type
    setTimeout(() => {
      // Focus on the specific resource slider
      const sliderElement = document.getElementById(`${resourceType}-filter-slider`);
      if (sliderElement) {
        sliderElement.focus();
      }
    }, 100);
  };
  
  // Format percentage for display
  const formatPercentage = (value) => {
    return `${value}%`;
  };

  // Function to get filter value by column ID
  const getFilterValue = (columnId) => {
    switch(columnId) {
      case 'cpu': return filters.cpu || 0;
      case 'memory': return filters.memory || 0;
      case 'disk': return filters.disk || 0;
      case 'download': return filters.download || 0;
      case 'upload': return filters.upload || 0;
      default: return 0;
    }
  };
  
  // Check if a resource has an active filter
  const hasActiveFilter = (columnId) => {
    return getFilterValue(columnId) > 0;
  };

  // Function to render the column content based on the column type
  const renderColumnContent = (column) => {
    // Check if this column is currently being hovered over
    const isHovered = hoveredColumn === column.id;
    
    // Check if this column is a resource column that can have thresholds
    const isResourceColumn = ['cpu', 'memory', 'disk', 'download', 'upload'].includes(column.id);
    
    // Check if this column has an active search filter
    const hasSearchFilter = activeFilteredColumns[column.id];
    
    // Common sort indicator component for all column types
    const SortIndicator = () => (
      <Box sx={{ 
        ml: 0.5, 
        display: 'flex', 
        alignItems: 'center',
        width: 20, // Fixed width for the sort indicator
        height: 20, // Fixed height
        visibility: sortConfig?.key === column.id ? 'visible' : 'hidden' // Hide but preserve space
      }}>
        {sortConfig?.key === column.id ? (
          sortConfig.direction === 'asc' ? (
            <KeyboardArrowUpIcon fontSize="small" />
          ) : (
            <KeyboardArrowDownIcon fontSize="small" />
          )
        ) : (
          // Invisible placeholder to reserve space
          <KeyboardArrowUpIcon fontSize="small" sx={{ opacity: 0 }} />
        )}
      </Box>
    );
      
    // Common column title styling
    const titleTypographyProps = {
      variant: "body2",
      sx: { 
                  fontWeight: sortConfig?.key === column.id || hasSearchFilter ? 600 : 400,
                  color: hasSearchFilter 
                    ? theme.palette.primary.main  
                    : sortConfig?.key === column.id 
                      ? theme.palette.mode === 'dark'
                        ? '#ffffff'
                        : theme.palette.primary.main
                      : 'inherit'
      }
    };

    // Common text click handler for ALL columns
    const handleTextClick = (e) => {
                  e.stopPropagation();
      
                // For resource columns, default to descending sort first time
      if (isResourceColumn && (!sortConfig || sortConfig.key !== column.id)) {
                  console.log(`Resource column ${column.id} clicked - defaulting to DESC sort`);
                  requestSort(column.id, 'desc');
      } else {
                  // First check if this is the currently sorted column
                  if (sortConfig && sortConfig.key === column.id) {
                    // If already sorted, explicitly toggle direction
                    const newDirection = sortConfig.direction === 'asc' ? 'desc' : 'asc';
                    console.log(`Column ${column.id} clicked - toggling from ${sortConfig.direction} to ${newDirection}`);
                    requestSort(column.id, newDirection);
                  } else {
                    // New column sort, use default toggle
                    console.log(`Column ${column.id} clicked - new sort`);
                    requestSort(column.id);
                  }
                }
    };
    
    // Render the appropriate column content
    return (
      <Box sx={{ 
        position: 'relative',
        width: '100%', 
        display: 'flex', 
        alignItems: 'center',
        justifyContent: 'space-between'
      }}>
        <Box 
                sx={{ 
                  display: 'flex',
                  alignItems: 'center',
            flexGrow: 1,
            cursor: 'pointer'
                }}
          onClick={handleTextClick}
              >
          {/* Removed search filter icon */}
          
          <Typography {...titleTypographyProps}>
                {column.label || column.id}
              </Typography>
          <SortIndicator />
          
          {/* Threshold indicator */}
          {hasActiveFilter(column.id) && (
            <Typography 
              variant="caption" 
              sx={{ 
                ml: 0.5, 
                color: theme.palette.primary.main,
                fontWeight: 500
              }}
            >
              ({formatPercentage(getFilterValue(column.id))}+)
            </Typography>
              )}
            </Box>
          </Box>
        );
  };

  return (
    <TableHead>
      <TableRow>
        {!hasVisibleColumns ? (
          <TableCell colSpan={12} align="center">
            <Box sx={{ 
              display: 'flex', 
              alignItems: 'center', 
              justifyContent: 'center',
              py: 1
            }}>
              <VisibilityOffIcon sx={{ mr: 1, color: 'text.disabled' }} />
              <Typography variant="body2" color="text.secondary" sx={{ mr: 2 }}>
                All columns are hidden
              </Typography>
              <Button 
                size="small" 
                variant="outlined" 
                onClick={() => {
                  console.log('Reset button clicked in NetworkTableHeader');
                  resetColumnVisibility();
                }}
              >
                Reset Columns
              </Button>
            </Box>
          </TableCell>
        ) : (
          <>
            {visibleColumns.map(column => (
              <TableCell 
                key={column.id}
                draggable={true}
                onDragStart={(e) => handleDragStart(e, column.id)}
                onDragOver={(e) => handleDragOver(e, column.id)}
                onDrop={(e) => handleDrop(e, column.id)}
                onDragEnd={handleDragEnd}
                onClick={(e) => {
                  // Only handle direct cell clicks - all columns now just sort, no dropdown functionality
                  if (e.target === e.currentTarget) {
                    const handleTextClick = (columnId) => {
                  // For resource columns, default to descending sort first time
                      const isResourceColumn = ['cpu', 'memory', 'disk', 'download', 'upload'].includes(columnId);
                      
                      if (isResourceColumn && (!sortConfig || sortConfig.key !== columnId)) {
                        console.log(`Resource column ${columnId} clicked - defaulting to DESC sort`);
                        requestSort(columnId, 'desc');
                      } else {
                    // First check if this is the currently sorted column
                        if (sortConfig && sortConfig.key === columnId) {
                      // If already sorted, explicitly toggle direction
                      const newDirection = sortConfig.direction === 'asc' ? 'desc' : 'asc';
                          console.log(`Column ${columnId} clicked - toggling from ${sortConfig.direction} to ${newDirection}`);
                          requestSort(columnId, newDirection);
                    } else {
                      // New column sort, use default toggle
                          console.log(`Column ${columnId} clicked - new sort`);
                          requestSort(columnId);
                    }
                      }
                    };
                    
                    // All columns handle sorting the same way
                    handleTextClick(column.id);
                  }
                }}
                onMouseEnter={handleColumnMouseEnter(column.id)}
                onMouseLeave={handleColumnMouseLeave}
                sx={{ 
                  width: getColumnWidth(column.id),
                  minWidth: `${getMinWidthForColumn(column.id)}px`,
                  maxWidth: autoSizeColumns.includes(column.id) ? '300px' : 'none',
                  padding: fixedNarrowColumns.includes(column.id) ? '0px 8px' : '16px 8px',
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  // Combined background color logic for all conditions
                  backgroundColor: draggedColumn === column.id 
                      ? alpha(theme.palette.primary.main, 0.15) // Use the darker shade from the active state
                      : hoveredColumn === column.id
                        ? alpha(theme.palette.primary.light, 0.1)
                        : dragOverColumn === column.id
                          ? alpha(theme.palette.primary.main, 0.05)
                          : alpha(theme.palette.primary.light, 0.05),
                  borderBottom: '1px solid',
                  borderBottomColor: 'divider',
                  borderTop: 'none',
                  boxShadow: draggedColumn === column.id 
                      ? `0 0 8px ${alpha(theme.palette.primary.main, 0.5)}` 
                      : 'none',
                  ...(fixedNarrowColumns.includes(column.id) && {
                    textAlign: 'center',
                    padding: '0px 8px'
                  }),
                  cursor: 'grab',
                  userSelect: 'auto',
                  position: 'relative',
                  transition: 'background-color 0.2s, transform 0.1s, box-shadow 0.2s',
                  
                  '& .drag-handle': {
                    visibility: 'visible',
                    opacity: 0.7
                  },
                  '&:active': {
                    cursor: 'grabbing',
                    backgroundColor: alpha(theme.palette.primary.main, 0.15)
                  }
                }}
              >
                <Box sx={{ 
                  display: 'flex', 
                  alignItems: 'center', 
                  justifyContent: 'space-between',
                  height: '100%'
                }}>
                  <Box sx={{ 
                    display: 'flex', 
                    alignItems: 'center', 
                    position: 'relative', 
                    width: '100%',
                    height: '100%'
                  }}>
                    {renderColumnContent(column)}
                  </Box>
                </Box>
              </TableCell>
            ))}
          </>
        )}
        
        <Menu
          id="node-filter-menu"
          anchorEl={nodeMenuAnchorEl}
          open={openNodeMenu}
          onClose={handleNodeMenuClose}
          MenuListProps={{
            'aria-labelledby': 'node-filter-button',
          }}
          anchorOrigin={{
            vertical: 'bottom',
            horizontal: 'left',
          }}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'left',
          }}
          PaperProps={{
            elevation: 3,
            sx: { 
              borderRadius: 2,
              overflow: 'hidden',
              width: 200,
              maxHeight: 300
            }
          }}
        >
          <Box sx={{ px: 2, py: 1, borderBottom: '1px solid', borderColor: 'divider' }}>
            <Typography variant="subtitle2" gutterBottom>
              Filter by Node
            </Typography>
          </Box>
          
          <Box sx={{ maxHeight: '60vh', overflow: 'auto' }}>
            {availableNodes.map((node) => (
              <MenuItem 
                key={node.id} 
                value={node.id}
                onClick={() => handleNodeSelect(node.id)}
                sx={{ 
                  py: 1,
                  borderLeft: selectedNode === node.id ? '3px solid' : 'none',
                  borderLeftColor: 'primary.main',
                  pl: selectedNode === node.id ? 1 : 2,
                }}
              >
                <ListItemIcon sx={{ minWidth: 36 }}>
                  {node.id === 'all' ? (
                    <ViewListIcon fontSize="small" color={selectedNode === node.id ? "primary" : "inherit"} />
                  ) : (
                    <ComputerIcon fontSize="small" color={selectedNode === node.id ? "primary" : "inherit"} />
                  )}
                </ListItemIcon>
                <ListItemText 
                  primary={node.name} 
                  primaryTypographyProps={{ 
                    variant: 'body2',
                    fontWeight: selectedNode === node.id ? 600 : 400
                  }} 
                />
                {selectedNode === node.id && (
                  <CheckIcon 
                    fontSize="small" 
                    color="primary" 
                    sx={{ ml: 1, fontSize: '1rem' }} 
                  />
                )}
              </MenuItem>
            ))}
          </Box>
        </Menu>
        
        <Menu
          id="status-filter-menu"
          anchorEl={statusMenuAnchorEl}
          open={openStatusMenu}
          onClose={handleStatusMenuClose}
          MenuListProps={{
            'aria-labelledby': 'status-filter-button',
          }}
          anchorOrigin={{
            vertical: 'bottom',
            horizontal: 'left',
          }}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'left',
          }}
          PaperProps={{
            elevation: 3,
            sx: { 
              borderRadius: 2,
              overflow: 'hidden',
              width: 200
            }
          }}
        >
          <Box sx={{ px: 2, py: 1, borderBottom: '1px solid', borderColor: 'divider' }}>
            <Typography variant="subtitle2" gutterBottom>
              Filter by Status
            </Typography>
          </Box>
          
          <MenuItem 
            value="all"
            onClick={() => handleStatusSelect('all')}
            sx={{ 
              py: 1,
              borderLeft: showStopped === null ? '3px solid' : 'none',
              borderLeftColor: 'primary.main',
              pl: showStopped === null ? 1 : 2,
            }}
          >
            <ListItemIcon sx={{ minWidth: 36 }}>
              <AllInclusiveIcon fontSize="small" color={showStopped === null ? "primary" : "inherit"} />
            </ListItemIcon>
            <ListItemText 
              primary="All Statuses" 
              primaryTypographyProps={{ 
                variant: 'body2',
                fontWeight: showStopped === null ? 600 : 400
              }} 
            />
            {showStopped === null && (
              <CheckIcon 
                fontSize="small" 
                color="primary" 
                sx={{ ml: 1, fontSize: '1rem' }} 
              />
            )}
          </MenuItem>
          
          <MenuItem 
            value="running"
            onClick={() => handleStatusSelect('running')}
            sx={{ 
              py: 1,
              borderLeft: showStopped === false ? '3px solid' : 'none',
              borderLeftColor: 'primary.main',
              pl: showStopped === false ? 1 : 2,
            }}
          >
            <ListItemIcon sx={{ minWidth: 36 }}>
              <PlayArrowIcon fontSize="small" color={showStopped === false ? "primary" : "inherit"} />
            </ListItemIcon>
            <ListItemText 
              primary="Running" 
              primaryTypographyProps={{ 
                variant: 'body2',
                fontWeight: showStopped === false ? 600 : 400
              }} 
            />
            {showStopped === false && (
              <CheckIcon 
                fontSize="small" 
                color="primary" 
                sx={{ ml: 1, fontSize: '1rem' }} 
              />
            )}
          </MenuItem>
          
          <MenuItem 
            value="stopped"
            onClick={() => handleStatusSelect('stopped')}
            sx={{ 
              py: 1,
              borderLeft: showStopped === true ? '3px solid' : 'none',
              borderLeftColor: 'primary.main',
              pl: showStopped === true ? 1 : 2,
            }}
          >
            <ListItemIcon sx={{ minWidth: 36 }}>
              <StopIcon fontSize="small" color={showStopped === true ? "primary" : "inherit"} />
            </ListItemIcon>
            <ListItemText 
              primary="Stopped" 
              primaryTypographyProps={{ 
                variant: 'body2',
                fontWeight: showStopped === true ? 600 : 400
              }} 
            />
            {showStopped === true && (
              <CheckIcon 
                fontSize="small" 
                color="primary" 
                sx={{ ml: 1, fontSize: '1rem' }} 
              />
            )}
          </MenuItem>
        </Menu>
        
        <Menu
          id="type-filter-menu"
          anchorEl={typeMenuAnchorEl}
          open={openTypeMenu}
          onClose={handleTypeMenuClose}
          MenuListProps={{
            'aria-labelledby': 'type-filter-button',
          }}
          anchorOrigin={{
            vertical: 'bottom',
            horizontal: 'left',
          }}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'left',
          }}
          PaperProps={{
            elevation: 3,
            sx: { 
              borderRadius: 2,
              overflow: 'hidden',
              width: 200
            }
          }}
        >
          <Box sx={{ px: 2, py: 1, borderBottom: '1px solid', borderColor: 'divider' }}>
            <Typography variant="subtitle2" gutterBottom>
              Filter by Type
            </Typography>
          </Box>
          
          <MenuItem 
            value="all"
            onClick={() => handleTypeSelect('all')}
            sx={{ 
              py: 1,
              borderLeft: guestTypeFilter === 'all' ? '3px solid' : 'none',
              borderLeftColor: 'primary.main',
              pl: guestTypeFilter === 'all' ? 1 : 2,
            }}
          >
            <ListItemIcon sx={{ minWidth: 36 }}>
              <ViewListIcon fontSize="small" color={guestTypeFilter === 'all' ? "primary" : "inherit"} />
            </ListItemIcon>
            <ListItemText 
              primary="All Types" 
              primaryTypographyProps={{ 
                variant: 'body2',
                fontWeight: guestTypeFilter === 'all' ? 600 : 400
              }} 
            />
            {guestTypeFilter === 'all' && (
              <CheckIcon 
                fontSize="small" 
                color="primary" 
                sx={{ ml: 1, fontSize: '1rem' }} 
              />
            )}
          </MenuItem>
          
          <MenuItem 
            value="vm"
            onClick={() => handleTypeSelect('vm')}
            sx={{ 
              py: 1,
              borderLeft: guestTypeFilter === 'vm' ? '3px solid' : 'none',
              borderLeftColor: 'primary.main',
              pl: guestTypeFilter === 'vm' ? 1 : 2,
            }}
          >
            <ListItemIcon sx={{ minWidth: 36 }}>
              <ComputerIcon fontSize="small" color={guestTypeFilter === 'vm' ? "primary" : "inherit"} />
            </ListItemIcon>
            <ListItemText 
              primary="VMs" 
              primaryTypographyProps={{ 
                variant: 'body2',
                fontWeight: guestTypeFilter === 'vm' ? 600 : 400
              }} 
            />
            {guestTypeFilter === 'vm' && (
              <CheckIcon 
                fontSize="small" 
                color="primary" 
                sx={{ ml: 1, fontSize: '1rem' }} 
              />
            )}
          </MenuItem>
          
          <MenuItem 
            value="ct"
            onClick={() => handleTypeSelect('ct')}
            sx={{ 
              py: 1,
              borderLeft: guestTypeFilter === 'ct' ? '3px solid' : 'none',
              borderLeftColor: 'primary.main',
              pl: guestTypeFilter === 'ct' ? 1 : 2,
            }}
          >
            <ListItemIcon sx={{ minWidth: 36 }}>
              <StorageIcon fontSize="small" color={guestTypeFilter === 'ct' ? "primary" : "inherit"} />
            </ListItemIcon>
            <ListItemText 
              primary="Containers" 
              primaryTypographyProps={{ 
                variant: 'body2',
                fontWeight: guestTypeFilter === 'ct' ? 600 : 400
              }} 
            />
            {guestTypeFilter === 'ct' && (
              <CheckIcon 
                fontSize="small" 
                color="primary" 
                sx={{ ml: 1, fontSize: '1rem' }} 
              />
            )}
          </MenuItem>
        </Menu>
        
        <Menu
          id="column-menu"
          anchorEl={columnMenuAnchorEl}
          open={openColumnMenu}
          onClose={handleColumnMenuClose}
          MenuListProps={{
            'aria-labelledby': 'column-visibility-button',
          }}
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
              overflow: 'hidden',
              width: 300,
              maxHeight: '80vh'
            }
          }}
          sx={{ zIndex: 1500 }}
        >
          <Box sx={{ px: 2, py: 1, borderBottom: '1px solid', borderColor: 'divider' }}>
            <Typography variant="subtitle2" gutterBottom>
              Column Visibility
            </Typography>
            <Typography variant="caption" color="text.secondary">
              Click to toggle visibility. Drag to reorder.
            </Typography>
          </Box>
          
          <Box sx={{ maxHeight: '60vh', overflow: 'auto' }}>
            {displayOrder.map((columnId) => {
              const config = columnVisibility[columnId];
              if (!config) return null;
              
              return (
                <MenuItem 
                  key={columnId}
                  onClick={() => toggleColumnVisibility(columnId)}
                  dense
                  className="column-menu-item"
                  data-column-id={columnId}
                  draggable={false}
                  onDragOver={(e) => {
                    e.preventDefault();
                    handleDragOver(e, columnId);
                  }}
                  onDrop={(e) => {
                    e.preventDefault();
                    handleDrop(e, columnId);
                  }}
                  onDragEnd={(e) => {
                    handleDragEnd();
                  }}
                  sx={{ 
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    backgroundColor: dragOverColumn === columnId 
                      ? 'rgba(25, 118, 210, 0.08)' 
                      : draggedColumn === columnId 
                        ? 'rgba(0, 0, 0, 0.04)' 
                        : 'transparent',
                    borderLeft: config.visible ? '3px solid' : 'none',
                    borderLeftColor: 'primary.main',
                    pl: config.visible ? 1 : 2,
                    cursor: draggedColumn === columnId ? 'grabbing' : 'pointer',
                    transition: 'all 0.2s ease',
                    position: 'relative',
                    transform: draggedColumn === columnId ? 'scale(1.02)' : 'scale(1)',
                    zIndex: draggedColumn === columnId ? 1200 : 1,
                    boxShadow: draggedColumn === columnId ? '0 2px 8px rgba(0,0,0,0.1)' : 'none',
                    '&:hover': {
                      backgroundColor: 'rgba(0, 0, 0, 0.04)'
                    },
                    '&:hover .drag-handle': {
                      opacity: 1
                    },
                    '&::before': dragOverColumn === columnId && draggedColumn !== columnId ? {
                      content: '""',
                      position: 'absolute',
                      left: 0,
                      right: 0,
                      height: '2px',
                      backgroundColor: 'primary.main',
                      top: getDropPosition(draggedColumn, columnId) === 'before' ? 0 : 'auto',
                      bottom: getDropPosition(draggedColumn, columnId) === 'after' ? 0 : 'auto',
                    } : {}
                  }}
                >
                  <Box sx={{ 
                    display: 'flex', 
                    alignItems: 'center', 
                    width: '100%',
                    justifyContent: 'space-between'
                  }}>
                    <Box sx={{ display: 'flex', alignItems: 'center' }}>
                      <Typography 
                        variant="body2"
                        sx={{
                          fontWeight: draggedColumn === columnId ? 500 : 400
                        }}
                      >
                        {config.label}
                      </Typography>
                    </Box>
                    <Box 
                      component="span" 
                      sx={{ 
                        width: 16, 
                        height: 16, 
                        borderRadius: '50%',
                        border: '1px solid',
                        borderColor: 'divider',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        bgcolor: config.visible ? 'primary.main' : 'transparent',
                        color: config.visible ? 'primary.contrastText' : 'transparent',
                      }}
                    >
                      {config.visible && <CheckIcon fontSize="small" sx={{ fontSize: 12 }} />}
                    </Box>
                  </Box>
                </MenuItem>
              );
            })}
            
            <Divider />
            <MenuItem onClick={resetColumnVisibility} sx={{ justifyContent: 'center' }}>
              <Typography variant="body2" color="primary">
                Reset to Default
              </Typography>
            </MenuItem>
          </Box>
        </Menu>
      </TableRow>
    </TableHead>
  );
};

// Update min width function to account for sorting icon
const getMinWidthForColumn = (columnId) => {
  const minWidths = {
    // Fixed narrow columns - add a few pixels to accommodate sort icon
    type: 50,      // Very small - just "VM" or "CT"
    id: 60,        // Very small - just numeric IDs
    status: 50,    // Minimal - just an icon
    
    // Auto-sized columns
    name: 120,     // Names - minimum width
    node: 80,      // Node names - minimum width (reduced from 100)
    
    // Flexible equal columns
    cpu: 140,      // Medium - progress bar with percentage
    memory: 140,   // Progress bar with byte values
    disk: 140,     // Progress bar with byte values
    
    // Fixed width columns
    download: 90,  // Network rates
    upload: 90,    // Network rates
    uptime: 90     // Time display
  };
  
  return minWidths[columnId] || 80;
};

export default NetworkTableHeader; 