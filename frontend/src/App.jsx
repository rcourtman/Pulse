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
import ViewCompactIcon from '@mui/icons-material/ViewCompact';
import NetworkDisplay from './components/NetworkDisplay';
import { AppThemeProvider, useThemeContext } from './context/ThemeContext';
import { SearchProvider } from './context/SearchContext';
import { UserSettingsProvider, useUserSettings } from './context/UserSettingsContext';
import SearchField from './components/SearchField';
import useSocket from './hooks/useSocket';
import { VERSION } from './utils/version';
import { AnimatedLogoWithText } from './components/network/UIComponents';
import { initializeStorage } from './utils/storageUtils';
import { SnackbarProvider } from 'notistack';

function AppContent() {
  // Initialize local storage with default values if needed
  useEffect(() => {
    initializeStorage();
  }, []);
  
  const theme = useTheme();
  const { status, connected } = useSocket();
  
  // Get theme context functions
  const { darkMode, toggleDarkMode } = useUserSettings();
  
  // Get user settings
  const { compactMode, toggleCompactMode } = useUserSettings();
  
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
  
  const [selectedNode, setSelectedNode] = useState('all');
  
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
      <AppBar position="static" color="primary" elevation={0}>
        <Toolbar sx={{ px: { xs: 2, sm: 3 } }}>
          <Box sx={{ display: 'flex', alignItems: 'center', flexGrow: 1 }}>
            <AnimatedLogoWithText darkMode={darkMode} />
            
            {/* Version display with link to repository */}
            <Tooltip title="View Releases">
              <Link 
                href={`https://github.com/rcourtman/pulse/releases/tag/v${VERSION}`}
                target="_blank" 
                rel="noopener noreferrer"
                sx={{ 
                  display: 'flex', 
                  alignItems: 'center',
                  ml: 2,
                  color: 'rgba(255, 255, 255, 0.7)',
                  textDecoration: 'none',
                  '&:hover': {
                    color: 'rgba(255, 255, 255, 1)',
                  }
                }}
              >
                <Typography 
                  variant="caption" 
                  sx={{ 
                    mr: 0.5,
                    display: { xs: 'none', sm: 'block' }
                  }}
                >
                  v{VERSION}
                </Typography>
                <GitHubIcon 
                  fontSize="small" 
                  sx={{ 
                    fontSize: '1rem',
                    opacity: 0.8,
                    '&:hover': {
                      opacity: 1
                    }
                  }}
                />
              </Link>
            </Tooltip>
          </Box>
          
          {/* Search Field */}
          <SearchField />
          
          {/* Column Visibility Button - Add reference to NetworkDisplay to access this */}
          <Box sx={{ display: { xs: 'none', sm: 'flex' }, mx: 1 }} id="column-visibility-app-header">
            {/* This Box will be populated by NetworkDisplay with the column visibility button */}
          </Box>
          
          {/* Settings Button */}
          <Tooltip title="Settings">
            <IconButton
              onClick={handleSettingsClick}
              size="small"
              color="inherit"
              aria-controls={isSettingsMenuOpen ? 'settings-menu' : undefined}
              aria-haspopup="true"
              aria-expanded={isSettingsMenuOpen ? 'true' : undefined}
              sx={{ 
                ml: 1,
                borderRadius: 2,
                p: { xs: 0.8, sm: 1 },
                bgcolor: 'rgba(255, 255, 255, 0.1)',
                '&:hover': {
                  bgcolor: 'rgba(255, 255, 255, 0.2)',
                },
              }}
            >
              <SettingsIcon />
            </IconButton>
          </Tooltip>
          
          {/* Settings Menu */}
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
            
            <MenuItem 
              onClick={toggleCompactMode}
              sx={{ py: 1 }}
            >
              <ListItemIcon>
                <ViewCompactIcon fontSize="small" color={compactMode ? "primary" : "inherit"} />
              </ListItemIcon>
              <ListItemText 
                primary="Compact Mode" 
                secondary={compactMode ? "Reduces row spacing" : "Standard row spacing"}
              />
              <Tooltip title={compactMode ? "Disable compact mode" : "Enable compact mode"}>
                <Switch 
                  edge="end" 
                  checked={compactMode}
                  size="small"
                  onClick={(e) => {
                    e.stopPropagation();
                    toggleCompactMode();
                  }}
                  color="primary"
                />
              </Tooltip>
            </MenuItem>
          </Menu>
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