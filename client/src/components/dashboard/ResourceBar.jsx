import React from 'react';

const ResourceBar = ({ value, type, label }) => {
  // Determine the color class based on type and value
  const getColorClass = () => {
    switch (type) {
      case 'cpu':
        return 'horizontal-bar-fill-cpu';
      case 'memory':
        return 'horizontal-bar-fill-memory';
      case 'network':
        return 'horizontal-bar-fill-network';
      case 'disk':
        return 'horizontal-bar-fill-disk';
      default:
        return 'bg-gray-500';
    }
  };
  
  // Determine the severity class based on value
  const getSeverityClass = () => {
    if (value >= 90) {
      return 'text-red-600 font-medium';
    } else if (value >= 70) {
      return 'text-orange-600';
    } else if (value >= 50) {
      return 'text-yellow-600';
    }
    return 'text-gray-600';
  };
  
  return (
    <div className="w-full">
      <div className="flex justify-between items-center mb-1">
        <div className={`text-xs ${getSeverityClass()}`}>
          {label}
        </div>
      </div>
      <div className="horizontal-bar w-full">
        <div
          className={getColorClass()}
          style={{ width: `${value}%` }}
        ></div>
      </div>
    </div>
  );
};

export default ResourceBar; 