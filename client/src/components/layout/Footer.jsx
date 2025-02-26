import { useState, useEffect } from 'react';
import { useLocation } from 'react-router-dom';

const Footer = () => {
  const location = useLocation();
  const [searchTerm, setSearchTerm] = useState('');
  const [filters, setFilters] = useState([]);
  
  // Only show search on dashboard
  const isDashboard = location.pathname === '/';
  
  // Handle search input change
  const handleSearchChange = (e) => {
    setSearchTerm(e.target.value);
    // Dispatch a custom event for live filtering
    window.dispatchEvent(
      new CustomEvent('pulse:search', { detail: e.target.value })
    );
  };
  
  // Handle search form submission
  const handleSearchSubmit = (e) => {
    e.preventDefault();
    if (searchTerm.trim() && !filters.includes(searchTerm.trim())) {
      const newFilters = [...filters, searchTerm.trim()];
      setFilters(newFilters);
      setSearchTerm('');
      
      // Dispatch a custom event for filter bubbles
      window.dispatchEvent(
        new CustomEvent('pulse:filters', { detail: newFilters })
      );
    }
  };
  
  // Remove a filter
  const removeFilter = (filter) => {
    const newFilters = filters.filter((f) => f !== filter);
    setFilters(newFilters);
    
    // Dispatch a custom event for filter bubbles
    window.dispatchEvent(
      new CustomEvent('pulse:filters', { detail: newFilters })
    );
  };
  
  // Reset filters when ESC is pressed
  useEffect(() => {
    const handleReset = () => {
      setSearchTerm('');
      setFilters([]);
    };
    
    window.addEventListener('pulse:reset', handleReset);
    
    return () => {
      window.removeEventListener('pulse:reset', handleReset);
    };
  }, []);
  
  return (
    <footer className="bg-white dark:bg-dark-card border-t border-gray-200 dark:border-gray-700 py-4">
      <div className="container mx-auto px-4">
        <div className="flex flex-col md:flex-row justify-between items-center">
          <div className="text-sm text-gray-500 dark:text-gray-400 mb-4 md:mb-0">
            &copy; {new Date().getFullYear()} Pulse. All rights reserved.
          </div>
          
          {isDashboard && (
            <div className="w-full md:w-auto">
              <form onSubmit={handleSearchSubmit} className="flex items-center">
                <input
                  type="text"
                  value={searchTerm}
                  onChange={handleSearchChange}
                  placeholder="Search resources..."
                  className="input w-full md:w-64"
                />
                <button
                  type="submit"
                  className="ml-2 btn btn-primary"
                  disabled={!searchTerm.trim()}
                >
                  Add Filter
                </button>
              </form>
              
              {filters.length > 0 && (
                <div className="mt-2 flex flex-wrap gap-2">
                  {filters.map((filter) => (
                    <div
                      key={filter}
                      className="bg-primary-100 dark:bg-primary-900 text-primary-800 dark:text-primary-200 px-3 py-1 rounded-full text-sm flex items-center"
                    >
                      {filter}
                      <button
                        onClick={() => removeFilter(filter)}
                        className="ml-2 text-primary-600 dark:text-primary-400 hover:text-primary-800 dark:hover:text-primary-200"
                      >
                        &times;
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </footer>
  );
};

export default Footer; 