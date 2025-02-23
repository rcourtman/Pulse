import React from 'react';
import { BrowserRouter } from 'react-router-dom';
import { NodeProvider } from './contexts/NodeContext';
import { WebSocketProvider } from './contexts/WebSocketContext';
import AppRoutes from './AppRoutes';

const App: React.FC = () => {
  console.log('App rendering'); // Debug log

  return (
    <BrowserRouter>
      <NodeProvider>
        <WebSocketProvider>
          <div className="min-h-screen bg-gray-100">
            <div className="container mx-auto px-4 py-8">
              <AppRoutes />
            </div>
          </div>
        </WebSocketProvider>
      </NodeProvider>
    </BrowserRouter>
  );
};

export default App;