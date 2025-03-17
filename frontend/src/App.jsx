import React, { useState, useEffect, Suspense, lazy } from 'react';
import { 
  Container, 
  Box, 
  Typography, 
  AppBar, 
  Toolbar, 
  Paper, 
  IconButton, 
  Tooltip, 
  alpha,
  Button,
  Link,
  Chip,
  Alert,
  Menu,
  MenuItem,
  ListItemIcon,
  ListItemText,
  Switch,
  Divider
} from '@mui/material';
import { useTheme } from '@mui/material/styles';
import Brightness4Icon from '@mui/icons-material/Brightness4';
import Brightness7Icon from '@mui/icons-material/Brightness7';
import GitHubIcon from '@mui/icons-material/GitHub';
import SettingsIcon from '@mui/icons-material/Settings';
import NetworkDisplay from './components/NetworkDisplay';
import { AppThemeProvider, useThemeContext } from './context/ThemeContext';
import { SearchProvider } from './context/SearchContext';
import { UserSettingsProvider, useUserSettings } from './context/UserSettingsContext';
import SearchField from './components/SearchField';
import NodeSelect from './components/NodeSelect';
import useSocket from './hooks/useSocket';
import { VERSION } from './utils/version';
import { AnimatedLogoWithText } from './components/network/UIComponents';
import { initializeStorage } from './utils/storageUtils';
import { SnackbarProvider } from 'notistack';
import PlayArrowIcon from '@mui/icons-material/PlayArrow';

function AppContent() {
  // Check if we're in development mode
  const isDevelopment = import.meta.env.DEV;
  
  // Initialize local storage with default values if needed
  useEffect(() => {
    initializeStorage();
  }, []);
  
  // Initialize storage based on environment
  useEffect(() => {
    // Check environment variables to decide if mock data should be enabled
    const envUseMockData = import.meta.env.VITE_USE_MOCK_DATA === 'true';
    const envMockDataEnabled = import.meta.env.VITE_MOCK_DATA_ENABLED === 'true';
    
    console.log(`Environment variables: USE_MOCK_DATA=${envUseMockData}, MOCK_DATA_ENABLED=${envMockDataEnabled}`);
    
    // Only use server-side mock data implementation
    if (isDevelopment) {
      if (envUseMockData || envMockDataEnabled) {
        console.log('Setting server-side mock data flags based on environment variables');
        localStorage.setItem('use_mock_data', 'true');
        localStorage.setItem('MOCK_DATA_ENABLED', 'true');
        
        // Clean up any client-side mock data settings
        localStorage.removeItem('mock_enabled');
        localStorage.removeItem('MOCK_SERVER_URL');
        localStorage.removeItem('MOCK_DATA');
        
        // Ensure we're not setting window globals for client-side mocking
        if (window.MOCK_DATA) {
          delete window.MOCK_DATA;
        }
      } else {
        console.log('Disabling mock data flags based on environment variables');
        localStorage.removeItem('use_mock_data');
        localStorage.removeItem('MOCK_DATA_ENABLED');
        localStorage.removeItem('mock_enabled');
        localStorage.removeItem('MOCK_SERVER_URL');
        localStorage.removeItem('MOCK_DATA');
      }
    } else {
      // In production, always ensure mock data is disabled
      if (localStorage.getItem('use_mock_data') === 'true' || localStorage.getItem('MOCK_DATA_ENABLED') === 'true') {
        console.log('Resetting mock data flags to match production environment');
        localStorage.removeItem('use_mock_data');
        localStorage.removeItem('MOCK_DATA_ENABLED');
        localStorage.removeItem('mock_enabled');
        localStorage.removeItem('MOCK_SERVER_URL');
        localStorage.removeItem('MOCK_DATA');
      }
    }
  }, [isDevelopment]);
  
  const theme = useTheme();
  const { status, connected, isMockData } = useSocket();
  
  // Get theme context functions
  const { darkMode, toggleDarkMode, showOnlyRunning, toggleShowOnlyRunning } = useUserSettings();
  
  // State for node selection
  const [selectedNode, setSelectedNode] = useState('all');
  
  // Listen for node change events from other components
  useEffect(() => {
    const handleNodeChange = (event) => {
      if (event.detail && event.detail.node) {
        setSelectedNode(event.detail.node);
      }
    };
    
    window.addEventListener('nodeChange', handleNodeChange);
    
    return () => {
      window.removeEventListener('nodeChange', handleNodeChange);
    };
  }, []);
  
  // State for settings menu
  const [settingsAnchorEl, setSettingsAnchorEl] = useState(null);
  const isSettingsMenuOpen = Boolean(settingsAnchorEl);
  
  // Settings menu handlers
  const handleSettingsClick = (event) => {
    setSettingsAnchorEl(event.currentTarget);
  };
  
  const handleSettingsClose = () => {
    setSettingsAnchorEl(null);
  };
  
  // Check if mock data is enabled - using both localStorage values and socket status
  const isMockDataEnabled = isMockData || 
    localStorage.getItem('use_mock_data') === 'true' || 
    localStorage.getItem('MOCK_DATA_ENABLED') === 'true' ||
    import.meta.env.VITE_FORCE_MOCK_DATA === 'true';
  
  // Generate content based on connection status
  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        minHeight: '100vh',
        bgcolor: 'background.default',
        color: 'text.primary',
      }}
    >
      {/* DEMO MODE Banner - Only visible when mock data is enabled */}
      {(isMockDataEnabled) && (
        <Alert 
          severity="warning" 
          variant="filled"
          sx={{
            borderRadius: 0,
            py: 0.5,
            bgcolor: theme.palette.mode === 'dark' 
              ? 'rgba(255, 152, 0, 0.7)' 
              : 'rgba(255, 152, 0, 0.85)',
            color: '#fff',
            fontWeight: 500,
            '& .MuiAlert-icon': {
              color: '#fff',
              alignItems: 'center'
            },
            '& .MuiAlert-message': {
              width: '100%',
              textAlign: 'center',
              fontSize: '0.95rem',
              textShadow: '0 1px 1px rgba(0,0,0,0.15)'
            }
          }}
        >
          Demo Mode - Running with simulated data
        </Alert>
      )}
      <AppBar position="static" color="primary" elevation={0}>
        <Toolbar sx={{ px: { xs: 2, sm: 3 } }}>
          <Box sx={{ display: 'flex', alignItems: 'center' }}>
            <AnimatedLogoWithText darkMode={darkMode} />
            
            {/* VERSION tag */}
            <Chip 
              size="small"
              label={`v${VERSION}`} 
              sx={{ 
                ml: 2,
                fontSize: '0.75rem',
                height: 24,
                borderRadius: 3,
                bgcolor: alpha(theme.palette.primary.main, theme.palette.mode === 'dark' ? 0.3 : 0.15),
                color: theme.palette.mode === 'dark' ? theme.palette.primary.light : theme.palette.primary.dark,
              }}
            />
          </Box>

          {/* Right-aligned actions */}
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, ml: 'auto' }}>
            {/* Search field */}
            <SearchField />

            {/* Node selection button */}
            <NodeSelect 
              selectedNode={selectedNode} 
              setSelectedNode={setSelectedNode} 
            />

            {/* Column Visibility Button - Add reference to NetworkDisplay to access this */}
            <Box sx={{ display: { xs: 'none', sm: 'flex' }, mx: 1 }} id="column-visibility-app-header">
              {/* This Box will be populated by NetworkDisplay with the column visibility button */}
            </Box>

            {/* GitHub link */}
            <Tooltip title="GitHub Repository">
              <IconButton 
                color="inherit" 
                aria-label="github"
                href="https://github.com/rcourtman/pulse"
                target="_blank"
                component="a"
                size="small"
                sx={{ ml: 0.5 }}
              >
                <GitHubIcon fontSize="small" />
              </IconButton>
            </Tooltip>

            {/* Settings menu */}
            <Tooltip title="Settings">
              <IconButton
                id="settings-button"
                aria-controls={isSettingsMenuOpen ? 'settings-menu' : undefined}
                aria-haspopup="true"
                aria-expanded={isSettingsMenuOpen ? 'true' : undefined}
                onClick={handleSettingsClick}
                color="inherit"
                size="small"
                sx={{ ml: 0.5 }}
              >
                <SettingsIcon fontSize="small" />
              </IconButton>
            </Tooltip>

            <Menu
              id="settings-menu"
              anchorEl={settingsAnchorEl}
              open={isSettingsMenuOpen}
              onClose={handleSettingsClose}
              MenuListProps={{
                'aria-labelledby': 'settings-button',
                dense: true,
              }}
              PaperProps={{
                elevation: 3,
                sx: {
                  mt: 1.5,
                  minWidth: 200,
                  borderRadius: 2,
                }
              }}
            >
              <MenuItem 
                onClick={toggleDarkMode}
                sx={{ py: 1 }}
              >
                <ListItemIcon>
                  {darkMode ? <Brightness7Icon fontSize="small" /> : <Brightness4Icon fontSize="small" />}
                </ListItemIcon>
                <ListItemText>
                  {darkMode ? "Switch to Light Mode" : "Switch to Dark Mode"}
                </ListItemText>
                <Switch 
                  edge="end" 
                  checked={darkMode}
                  size="small"
                  onClick={(e) => {
                    e.stopPropagation();
                    toggleDarkMode();
                  }}
                />
              </MenuItem>
              
              <Divider />
              
              {/* Show only running guests toggle */}
              <MenuItem 
                onClick={toggleShowOnlyRunning}
                sx={{ py: 1 }}
              >
                <ListItemIcon>
                  <PlayArrowIcon fontSize="small" color={showOnlyRunning ? "primary" : "inherit"} />
                </ListItemIcon>
                <ListItemText 
                  primary="Show Only Running"
                  secondary={showOnlyRunning ? "Hiding stopped guests" : "Showing all guests"}
                />
                <Switch 
                  edge="end" 
                  checked={showOnlyRunning}
                  size="small"
                  onClick={(e) => {
                    e.stopPropagation();
                    toggleShowOnlyRunning();
                  }}
                  color="primary"
                />
              </MenuItem>
            </Menu>
          </Box>
        </Toolbar>
      </AppBar>
      
      <Container 
        maxWidth={false} 
        sx={{ 
          mt: { xs: 0.5, sm: 0.5 }, 
          mb: { xs: 2, sm: 4 }, 
          flexGrow: 1,
          px: { xs: 1, sm: 2 },
          width: '100%',
          mx: 'auto'
        }}
        onClick={(e) => {
          if (e.target === e.currentTarget) {
            document.activeElement?.blur();
          }
        }}
      >
        <Box sx={{ my: { xs: 0.5, sm: 0.5 } }}>
          {/* Network Display Component - pass the selected node as a prop */}
          <NetworkDisplay selectedNode={selectedNode} />
        </Box>
      </Container>
      
      <Paper 
        sx={{ 
          padding: 2, 
          marginTop: 'auto',
          borderRadius: 0,
          bgcolor: 'background.paper',
          borderTop: '1px solid',
          borderColor: 'divider',
        }} 
        component="footer"
        elevation={0}
      >
        <Typography variant="body2" color="text.secondary" align="center">
          Pulse © {new Date().getFullYear()}
        </Typography>
      </Paper>
    </Box>
  );
}

function App() {
  useEffect(() => {
    // Sync important environment variables to localStorage
    if (import.meta.env.VITE_MOCK_CLUSTER_ENABLED !== undefined) {
      localStorage.setItem('MOCK_CLUSTER_ENABLED', import.meta.env.VITE_MOCK_CLUSTER_ENABLED);
    }
    if (import.meta.env.VITE_MOCK_DATA_ENABLED !== undefined) {
      localStorage.setItem('MOCK_DATA_ENABLED', import.meta.env.VITE_MOCK_DATA_ENABLED);
    }
    if (import.meta.env.VITE_PROXMOX_AUTO_DETECT_CLUSTER !== undefined) {
      localStorage.setItem('PROXMOX_AUTO_DETECT_CLUSTER', import.meta.env.VITE_PROXMOX_AUTO_DETECT_CLUSTER);
    }
    
    // Check for cluster status from server directly when application loads
    fetch('/api/cluster/status')
      .then(response => response.json())
      .then(data => {
        const hasCluster = data?.hasCluster || false;
        console.log('Initial cluster status check:', { hasCluster, data });
        
        // Update localStorage based on actual server detection
        localStorage.setItem('CLUSTER_DETECTED', hasCluster ? 'true' : 'false');
        
        // If this is a non-cluster environment, force remove column visibility setting
        // This ensures the HA column is hidden
        if (!hasCluster) {
          // Clear the column visibility to reset to defaults
          localStorage.removeItem('network_display_column_visibility');
          
          // Set an explicit column visibility object with role hidden
          const defaultVisibility = {
            name: { id: 'name', label: 'Name', visible: true },
            status: { id: 'status', label: 'Status', visible: true },
            node: { id: 'node', label: 'Node', visible: false },
            role: { id: 'role', label: 'HA Status', visible: false },
            type: { id: 'type', label: 'Type', visible: true },
            id: { id: 'id', label: 'ID', visible: true },
            cpu: { id: 'cpu', label: 'CPU', visible: true },
            memory: { id: 'memory', label: 'Memory', visible: true },
            disk: { id: 'disk', label: 'Disk', visible: true },
            download: { id: 'download', label: 'Download', visible: true },
            upload: { id: 'upload', label: 'Upload', visible: true },
            uptime: { id: 'uptime', label: 'Uptime', visible: true }
          };
          
          // Save this explicit setting to localStorage
          localStorage.setItem('network_display_column_visibility', JSON.stringify(defaultVisibility));
          console.log('Non-cluster environment detected, forced HA Status column to hide');
        }
        
        // Store current status for future comparison
        localStorage.setItem('previous_cluster_status', hasCluster ? 'true' : 'false');
      })
      .catch(error => {
        console.warn('Unable to check cluster status on load:', error);
        // Default to no cluster if server check fails
        localStorage.setItem('CLUSTER_DETECTED', 'false');
      });
  }, []);

  return (
    <AppThemeProvider>
      <SearchProvider>
        <UserSettingsProvider>
          <SnackbarProvider maxSnack={3}>
            <AppContent />
          </SnackbarProvider>
        </UserSettingsProvider>
      </SearchProvider>
    </AppThemeProvider>
  );
}

export default App; 