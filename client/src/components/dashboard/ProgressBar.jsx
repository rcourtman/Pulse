import React from 'react';
import PropTypes from 'prop-types';

/**
 * ProgressBar component for displaying resource usage
 * Works with both raw Proxmox data and processed data
 */
const ProgressBar = ({ 
  value = 0, 
  label = '0%', 
  thresholds = { warning: 70, critical: 90 } 
}) => {
  // Ensure value is a number and between 0-100
  const numericValue = typeof value === 'string' ? parseFloat(value) : Number(value) || 0;
  const normalizedValue = Math.min(Math.max(0, numericValue), 100);
  
  // Determine color based on thresholds
  let barColor = 'bg-green-500 dark:bg-green-600';
  
  if (normalizedValue >= thresholds.critical) {
    barColor = 'bg-red-500 dark:bg-red-600';
  } else if (normalizedValue >= thresholds.warning) {
    barColor = 'bg-yellow-500 dark:bg-yellow-600';
  }
  
  return (
    <div className="w-full">
      <div className="flex justify-between items-center mb-1">
        <span className="text-xs font-medium text-gray-700 dark:text-gray-300">{label}</span>
      </div>
      <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2.5">
        <div 
          className={`${barColor} h-2.5 rounded-full transition-all duration-500`} 
          style={{ width: `${normalizedValue}%` }}
        ></div>
      </div>
    </div>
  );
};

ProgressBar.propTypes = {
  // Value as a percentage (0-100) or string that can be parsed to a number
  value: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  // Label to display (e.g. "50%")
  label: PropTypes.string,
  // Thresholds for color changes
  thresholds: PropTypes.shape({
    warning: PropTypes.number,
    critical: PropTypes.number
  })
};

export default ProgressBar; 