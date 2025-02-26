import { Link, useLocation } from 'react-router-dom';
import { useServersStore } from '../../contexts/ServersStore';
import { useTheme } from '../../contexts/ThemeContext';
import { FaBug } from 'react-icons/fa';
import { MdLightMode, MdDarkMode } from 'react-icons/md';

const Header = () => {
  const location = useLocation();
  const { servers } = useServersStore();
  const { darkMode, toggleDarkMode } = useTheme();
  
  // Don't show navigation during onboarding
  const isOnboarding = location.pathname === '/onboarding';
  // Check if we're in development mode
  const isDevelopment = import.meta.env.DEV;
  
  return (
    <header className="bg-white dark:bg-dark-card shadow">
      <div className="container mx-auto px-4 py-4">
        <div className="flex justify-between items-center">
          <div className="flex items-center">
            <Link to="/" className="text-2xl font-bold text-primary-600">
              Pulse
            </Link>
            <span className="ml-2 text-xs bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded-full text-gray-600 dark:text-gray-300">
              v0.1.0
            </span>
          </div>
          
          <div className="flex items-center">
            {!isOnboarding && (
              <nav className="flex space-x-4 mr-4">
                <Link
                  to="/"
                  className={`px-3 py-2 rounded-md ${
                    location.pathname === '/'
                      ? 'bg-primary-50 dark:bg-primary-900 text-primary-700 dark:text-primary-300'
                      : 'text-gray-600 dark:text-gray-300 hover:text-primary-600 dark:hover:text-primary-400'
                  }`}
                >
                  Dashboard
                </Link>
                <Link
                  to="/servers"
                  className={`px-3 py-2 rounded-md ${
                    location.pathname === '/servers'
                      ? 'bg-primary-50 dark:bg-primary-900 text-primary-700 dark:text-primary-300'
                      : 'text-gray-600 dark:text-gray-300 hover:text-primary-600 dark:hover:text-primary-400'
                  }`}
                >
                  Servers
                  {servers.length > 0 && (
                    <span className="ml-2 bg-primary-100 dark:bg-primary-800 text-primary-800 dark:text-primary-200 text-xs px-2 py-0.5 rounded-full">
                      {servers.length}
                    </span>
                  )}
                </Link>
                {/* Only show Debug link in development mode */}
                {isDevelopment && (
                  <Link
                    to="/debug"
                    className={`px-3 py-2 rounded-md flex items-center ${
                      location.pathname === '/debug'
                        ? 'bg-primary-50 dark:bg-primary-900 text-primary-700 dark:text-primary-300'
                        : 'text-gray-600 dark:text-gray-300 hover:text-primary-600 dark:hover:text-primary-400'
                    }`}
                  >
                    <FaBug className="mr-1" />
                    Debug
                  </Link>
                )}
              </nav>
            )}
            
            {/* Theme toggle button */}
            <button
              onClick={toggleDarkMode}
              className="p-2 rounded-full hover:bg-gray-100 dark:hover:bg-gray-700 focus:outline-none transition-colors"
              aria-label={darkMode ? "Switch to light mode" : "Switch to dark mode"}
            >
              {darkMode ? (
                <MdLightMode className="h-5 w-5 text-amber-500" />
              ) : (
                <MdDarkMode className="h-5 w-5 text-gray-700" />
              )}
            </button>
          </div>
        </div>
      </div>
    </header>
  );
};

export default Header; 