import React from 'react';
import { 
  Box, 
  Typography, 
  LinearProgress, 
  Tooltip,
  alpha,
  useTheme
} from '@mui/material';
import { formatPercentage } from '../../utils/formatters';
import { pulseAnimation, logoPulseAnimation, centerGlowAnimation } from '../../constants/networkConstants';

// Animated Logo component
export const AnimatedLogo = React.memo(({ size = 32, darkMode = false }) => {
  return (
    <Box
      sx={{
        position: 'relative',
        width: size,
        height: size,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        marginRight: 2,
      }}
    >
      {/* Background circle with subtle pulse */}
      <Box
        sx={{
          position: 'absolute',
          width: '100%',
          height: '100%',
          borderRadius: '50%',
          background: 'linear-gradient(135deg, rgba(58, 123, 213, 0.8), rgba(58, 123, 213, 0.7))',
          filter: darkMode ? 'brightness(1.2)' : 'none',
          animation: `${pulseAnimation} 4s infinite ease-in-out`,
          animationDelay: '0.5s',
        }}
      />
      
      {/* Outer ring */}
      <Box
        sx={{
          position: 'absolute',
          width: '95%',
          height: '95%',
          borderRadius: '50%',
          background: 'linear-gradient(135deg, rgba(255, 255, 255, 0.2), rgba(255, 255, 255, 0.1))',
        }}
      />
      
      {/* Pulse ring (animated) */}
      <Box
        sx={{
          position: 'absolute',
          width: '70%',
          height: '70%',
          borderRadius: '50%',
          border: '2px solid rgba(255, 255, 255, 0.7)',
          zIndex: 1,
          animation: `${logoPulseAnimation} 2.5s infinite ease-in-out`,
        }}
      />
      
      {/* Center dot */}
      <Box
        sx={{
          position: 'absolute',
          width: '30%',
          height: '30%',
          borderRadius: '50%',
          background: 'linear-gradient(135deg, #ffffff, #f0f0f0)',
          animation: `${centerGlowAnimation} 3s infinite ease-in-out`,
          animationDelay: '0.2s',
          zIndex: 2,
        }}
      />
    </Box>
  );
});

// Animated Logo with Text component
export const AnimatedLogoWithText = React.memo(({ size = 32, darkMode = false }) => {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center' }}>
      <AnimatedLogo size={size} darkMode={darkMode} />
      <Typography 
        variant="h6" 
        component="div" 
        sx={{ 
          fontWeight: 'bold',
          color: darkMode ? 'white' : 'inherit',
          letterSpacing: '0.5px',
          textShadow: darkMode 
            ? '0 2px 4px rgba(0, 0, 0, 0.5), 0 0 2px rgba(0, 0, 0, 0.3)' 
            : '0 2px 4px rgba(0, 0, 0, 0.3), 0 0 2px rgba(0, 0, 0, 0.2)'
        }}
      >
        Pulse
      </Typography>
    </Box>
  );
});

// Progress bar with label and tooltip
export const ProgressWithLabel = React.memo(({ value, color = "primary", disabled = false, tooltipText }) => {
  // Ensure value is a number and between 0-100
  const normalizedValue = typeof value === 'number' 
    ? Math.min(Math.max(0, value), 100) 
    : 0;
  
  const progressBar = (
    <Box sx={{ 
      display: 'flex', 
      alignItems: 'center', 
      width: '100%',
      opacity: disabled ? 0.5 : 1,
    }}>
      <Box sx={{ minWidth: 35, mr: 1 }}>
        <Typography 
          variant="body2" 
          color="text.secondary"
          sx={{ 
            fontWeight: 500,
            fontSize: '0.75rem',
            whiteSpace: 'nowrap',
          }}
        >
          {formatPercentage(normalizedValue)}
        </Typography>
      </Box>
      <Box sx={{ width: '100%' }}>
        <LinearProgress 
          variant="determinate" 
          value={normalizedValue} 
          color={color}
          sx={{
            height: 4,
            borderRadius: 4,
            backgroundColor: theme => alpha(theme.palette.grey[300], 0.5),
            '& .MuiLinearProgress-bar': {
              borderRadius: 4,
              transition: 'transform 0.4s cubic-bezier(0.4, 0, 0.2, 1)',
            }
          }}
        />
      </Box>
    </Box>
  );
  
  if (tooltipText) {
    return (
      <Tooltip title={tooltipText} arrow placement="top">
        {progressBar}
      </Tooltip>
    );
  }
  
  return progressBar;
});

// Status indicator circle
export const StatusIndicator = React.memo(({ status }) => {
  const theme = useTheme();
  const isDarkMode = theme.palette.mode === 'dark';
  let color = 'grey';
  
  switch (status.toLowerCase()) {
    case 'running':
      color = '#4caf50'; // success green
      break;
    case 'stopped':
      color = '#f44336'; // error red
      break;
    case 'paused':
      color = '#ff9800'; // warning orange
      break;
    default:
      color = '#9e9e9e'; // grey
  }
  
  return (
    <Box
      sx={{
        width: 8,
        height: 8,
        borderRadius: '50%',
        bgcolor: color,
        boxShadow: isDarkMode 
          ? `0 0 0 1px ${alpha(color, 0.5)}` 
          : '0 0 0 1px rgba(255, 255, 255, 0.8)',
        ...(status.toLowerCase() === 'running' && {
          // No animation for running status
        })
      }}
    />
  );
});

// KeyboardShortcut component with better a11y
export const KeyboardShortcut = ({ shortcut, sx = {} }) => (
  <Box
    component="span"
    sx={{
      display: 'inline-flex', 
      alignItems: 'center', 
      justifyContent: 'center',
      px: 0.7,
      py: 0.3,
      ml: 0.8,
      fontSize: '0.65rem',
      lineHeight: 1,
      fontWeight: 600,
      color: 'text.secondary',
      bgcolor: 'rgba(0, 0, 0, 0.06)',
      borderRadius: 0.8,
      border: '1px solid',
      borderColor: 'rgba(0, 0, 0, 0.1)',
      ...sx
    }}
    aria-hidden="true" // Hide from screen readers since it's visual decoration
  >
    {shortcut}
  </Box>
);

// Add a new HighlightedText component for search term highlighting
export const HighlightedText = ({ text, searchTerms = [], variant = "body2", sx = {}, ...props }) => {
  const theme = useTheme();
  
  if (!text || !searchTerms || searchTerms.length === 0) {
    return <Typography variant={variant} sx={sx} {...props}>{text}</Typography>;
  }
  
  // Convert to string to handle numeric values
  const textStr = String(text || '');
  
  // Prepare regular expression for matching - case insensitive
  // Escape special regex characters in search terms
  const escapedTerms = searchTerms.map(term => 
    String(term).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  );
  
  // Create regex pattern from all terms
  const pattern = new RegExp(`(${escapedTerms.join('|')})`, 'gi');
  
  // Split text by matches
  const parts = textStr.split(pattern);
  
  // Skip highlighting if no matches
  if (parts.length <= 1) {
    return <Typography variant={variant} sx={sx} {...props}>{text}</Typography>;
  }

  return (
    <Typography variant={variant} sx={sx} {...props}>
      {parts.map((part, i) => {
        // Check if this part matches any search term (case insensitive)
        const isMatch = searchTerms.some(term => 
          part.toLowerCase() === String(term).toLowerCase()
        );
        
        return isMatch ? (
          <Box 
            component="span" 
            key={i}
            sx={{ 
              backgroundColor: alpha(theme.palette.primary.main, 0.2),
              borderRadius: '2px',
              padding: '0 2px',
              fontWeight: 'medium'
            }}
          >
            {part}
          </Box>
        ) : part;
      })}
    </Typography>
  );
}; 