import { useEffect, useState } from 'react';
import { Routes, Route, Navigate, useLocation } from 'react-router-dom';
import { useServersStore } from './contexts/ServersStore';
import useConfiguredNodes from './hooks/useConfiguredNodes';
import Layout from './components/layout/Layout';
import Dashboard from './pages/Dashboard';
import Onboarding from './pages/Onboarding';
import ServerManagement from './pages/ServerManagement';
// Only import Debug in development mode
const Debug = import.meta.env.DEV ? import('./pages/Debug').then(module => module.default) : null;
import NotFound from './pages/NotFound';

function App() {
  const { servers, initialized, setInitialized } = useServersStore();
  const { nodes: configuredNodes, isLoading: isLoadingNodes, error: nodesError } = useConfiguredNodes();
  const location = useLocation();
  const [isCheckingNodes, setIsCheckingNodes] = useState(true);
  // State to track if Debug component is loaded
  const [DebugComponent, setDebugComponent] = useState(null);
  
  // Load Debug component in development mode
  useEffect(() => {
    if (import.meta.env.DEV && Debug) {
      Debug.then(Component => {
        setDebugComponent(() => Component);
      });
    }
  }, []);

  // Initialize the store
  useEffect(() => {
    setInitialized(true);
  }, [setInitialized]);

  // Wait for nodes to load
  useEffect(() => {
    if (!isLoadingNodes) {
      setIsCheckingNodes(false);
      console.log('Nodes loaded:', configuredNodes);
    }
  }, [isLoadingNodes, configuredNodes]);

  // If not initialized yet or still checking nodes, show loading
  if (!initialized || isCheckingNodes) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-pulse-slow text-2xl font-semibold text-primary-600">
          Loading Pulse...
        </div>
      </div>
    );
  }

  // If we're on the debug page and in development mode, allow access
  if (import.meta.env.DEV && location.pathname === '/debug' && DebugComponent) {
    return (
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route path="debug" element={<DebugComponent />} />
          <Route path="*" element={<Navigate to="/debug" replace />} />
        </Route>
      </Routes>
    );
  }

  // If no servers are configured (either in localStorage or from server), redirect to onboarding
  // EXCEPT if we're already on the onboarding page
  const hasServers = servers.length > 0 || configuredNodes.length > 0;
  const isOnboardingPage = location.pathname === '/onboarding';
  
  // Only log in development mode
  if (import.meta.env.DEV) {
    console.log('Servers from store:', servers);
    console.log('Configured nodes from server:', configuredNodes);
    console.log('Has servers:', hasServers);
    console.log('Current path:', location.pathname);
    console.log('Is onboarding page:', isOnboardingPage);
  }

  // If we have no servers and we're not on the onboarding page, redirect to onboarding
  if (!hasServers && !isOnboardingPage) {
    console.log('No servers configured, redirecting to onboarding');
    return <Navigate to="/onboarding" replace />;
  }

  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<Dashboard />} />
        <Route path="onboarding" element={<Onboarding />} />
        <Route path="servers" element={<ServerManagement />} />
        {/* Only include Debug route in development mode */}
        {import.meta.env.DEV && DebugComponent && (
          <Route path="debug" element={<DebugComponent />} />
        )}
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  );
}

export default App; 