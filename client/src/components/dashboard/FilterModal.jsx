import React, { useState } from 'react';

const FilterModal = ({ filters, onApplyFilters, onClose }) => {
  const [localFilters, setLocalFilters] = useState(filters || []);
  
  const handleAddFilter = (filter) => {
    setLocalFilters([...localFilters, filter]);
  };
  
  const handleRemoveFilter = (index) => {
    const newFilters = [...localFilters];
    newFilters.splice(index, 1);
    setLocalFilters(newFilters);
  };
  
  const handleApply = () => {
    onApplyFilters(localFilters);
    onClose();
  };
  
  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl p-6 w-full max-w-md">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-xl font-semibold">Filter Resources</h2>
          <button
            onClick={onClose}
            className="text-gray-500 hover:text-gray-700"
          >
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path>
            </svg>
          </button>
        </div>
        
        <div className="mb-4">
          <div className="flex flex-wrap gap-2 mb-2">
            {localFilters.map((filter, index) => (
              <div key={index} className="bg-gray-100 rounded-full px-3 py-1 flex items-center">
                <span className="text-sm">{filter}</span>
                <button
                  onClick={() => handleRemoveFilter(index)}
                  className="ml-2 text-gray-500 hover:text-gray-700"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path>
                  </svg>
                </button>
              </div>
            ))}
          </div>
          
          <div className="flex">
            <input
              type="text"
              placeholder="Add a filter tag..."
              className="flex-1 px-3 py-2 border border-gray-300 rounded-l-lg focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
              onKeyDown={(e) => {
                if (e.key === 'Enter' && e.target.value.trim()) {
                  handleAddFilter(e.target.value.trim());
                  e.target.value = '';
                }
              }}
            />
            <button
              onClick={(e) => {
                const input = e.target.previousSibling;
                if (input.value.trim()) {
                  handleAddFilter(input.value.trim());
                  input.value = '';
                }
              }}
              className="px-4 py-2 bg-primary-600 text-white rounded-r-lg hover:bg-primary-700 transition-colors"
            >
              Add
            </button>
          </div>
        </div>
        
        <div className="flex justify-end space-x-2">
          <button
            onClick={onClose}
            className="px-4 py-2 border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleApply}
            className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
          >
            Apply Filters
          </button>
        </div>
      </div>
    </div>
  );
};

export default FilterModal; 