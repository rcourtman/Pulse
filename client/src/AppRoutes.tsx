import React from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import Dashboard from './pages/Dashboard';
import NodeDetails from './pages/NodeDetails';

const AppRoutes: React.FC = () => {
  return (
    <>
      <h1 className="text-3xl font-bold text-gray-900 mb-8">
        Pulse - Proxmox Monitoring Dashboard
      </h1>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/nodes/:nodeId" element={<NodeDetails />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </>
  );
};

export default AppRoutes;