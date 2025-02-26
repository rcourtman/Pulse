import React from 'react';
import { FaSort, FaSortUp, FaSortDown } from 'react-icons/fa';

const SortControls = ({ sortConfig, onSortChange }) => {
  const sortOptions = [
    { key: 'name', label: 'Name' },
    { key: 'cpu_usage', label: 'CPU' },
    { key: 'memory_used', label: 'Memory' },
    { key: 'status', label: 'Status' }
  ];

  const getSortIcon = (key) => {
    if (sortConfig.key !== key) return <FaSort />;
    return sortConfig.direction === 'asc' ? <FaSortUp /> : <FaSortDown />;
  };

  const handleSort = (key) => {
    let direction = 'asc';
    if (sortConfig.key === key) {
      direction = sortConfig.direction === 'asc' ? 'desc' : 'asc';
    }
    onSortChange({ key, direction });
  };

  return (
    <div className="sort-controls">
      <span className="sort-label">Sort by:</span>
      <div className="sort-buttons">
        {sortOptions.map(option => (
          <button
            key={option.key}
            className={`sort-button ${sortConfig.key === option.key ? 'active' : ''}`}
            onClick={() => handleSort(option.key)}
          >
            {option.label} {getSortIcon(option.key)}
          </button>
        ))}
      </div>
    </div>
  );
};

export default SortControls; 