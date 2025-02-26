import { Outlet } from 'react-router-dom';
import Header from './Header';
import Footer from './Footer';
import { useEffect } from 'react';
import { useTheme } from '../../contexts/ThemeContext';

const Layout = () => {
  const { darkMode } = useTheme();

  // Handle ESC key to reset filters and sorting
  useEffect(() => {
    const handleKeyDown = (e) => {
      if (e.key === 'Escape') {
        // Dispatch a custom event that components can listen for
        window.dispatchEvent(new CustomEvent('pulse:reset'));
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  return (
    <div className={`flex flex-col min-h-screen ${darkMode ? 'dark:bg-dark-background dark:text-dark-foreground' : 'bg-background text-foreground'}`}>
      <Header />
      <main className="flex-grow container mx-auto px-4 py-6">
        <Outlet />
      </main>
      <Footer />
    </div>
  );
};

export default Layout; 