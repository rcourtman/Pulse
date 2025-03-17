import React, { useState, useEffect, useMemo } from 'react';
import {
  Button,
  Menu,
  MenuItem,
  ListItemIcon,
  ListItemText,
  Box,
  Typography,
  Tooltip
} from '@mui/material';
import ComputerIcon from '@mui/icons-material/Computer';
import ViewListIcon from '@mui/icons-material/ViewList';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import CheckIcon from '@mui/icons-material/Check';
import useSocket from '../hooks/useSocket';

const NodeSelect = ({ selectedNode = 'all', setSelectedNode }) => {
  const [nodeMenuAnchorEl, setNodeMenuAnchorEl] = useState(null);
  const openNodeMenu = Boolean(nodeMenuAnchorEl);
  
  // Get node data from socket connection
  const { nodeData, guestData } = useSocket();

  // Prepare available nodes for dropdown
  const availableNodes = useMemo(() => {
    // Start with the "All Nodes" option
    const nodes = [
      { id: 'all', name: 'All Nodes', count: 0 }
    ];
    
    // Add actual nodes if they exist
    if (nodeData && nodeData.length > 0) {
      // Add real nodes from the API
      nodeData.forEach(node => {
        nodes.push({
          id: node.id,
          name: node.name || node.id,
          count: 0
        });
      });
    } 
    // If no node data, try to extract nodes from guest data
    else if (guestData && guestData.length > 0) {
      // Collect all unique node names from guests
      const nodeSet = new Set();
      guestData.forEach(guest => {
        if (guest.node) {
          nodeSet.add(guest.node);
        }
      });
      
      // Create node objects from the unique node names
      Array.from(nodeSet).forEach(nodeName => {
        nodes.push({
          id: nodeName,
          name: nodeName,
          count: 0
        });
      });
    }
    
    return nodes;
  }, [nodeData, guestData]);

  // Handle node menu button click
  const handleNodeMenuClick = (event) => {
    setNodeMenuAnchorEl(event.currentTarget);
  };

  // Handle node menu close
  const handleNodeMenuClose = () => {
    setNodeMenuAnchorEl(null);
  };

  // Handle node selection
  const handleNodeSelect = (nodeId) => {
    setSelectedNode(nodeId);
    handleNodeMenuClose();
  };

  // Get the name of the currently selected node
  const selectedNodeName = useMemo(() => {
    const node = availableNodes.find(n => n.id === selectedNode);
    return node ? node.name : selectedNode;
  }, [availableNodes, selectedNode]);

  return (
    <>
      <Tooltip title="Filter by node">
        <Button
          id="node-filter-button"
          aria-controls={openNodeMenu ? 'node-filter-menu' : undefined}
          aria-haspopup="true"
          aria-expanded={openNodeMenu ? 'true' : undefined}
          onClick={handleNodeMenuClick}
          endIcon={<ExpandMoreIcon />}
          color="inherit"
          size="small"
          sx={{ 
            ml: 1, 
            textTransform: 'none',
            fontSize: '0.85rem'
          }}
        >
          {selectedNodeName}
        </Button>
      </Tooltip>

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
    </>
  );
};

export default NodeSelect; 