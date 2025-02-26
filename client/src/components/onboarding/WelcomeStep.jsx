import React from 'react';
import { FaServer, FaKey, FaChartBar } from 'react-icons/fa';

const WelcomeStep = ({ nextStep }) => {
  return (
    <div className="text-center">
      <h2 className="text-2xl font-semibold mb-4 dark:text-white">Welcome to Pulse</h2>
      <p className="text-gray-600 dark:text-gray-300 mb-8">
        Let's set up your Proxmox server monitoring. This wizard will guide you through:
      </p>
      
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        <div className="p-4 border border-gray-200 dark:border-gray-700 rounded-lg dark:bg-gray-800">
          <div className="flex justify-center mb-4">
            <FaServer className="text-4xl text-primary-500" />
          </div>
          <h3 className="font-medium mb-2 dark:text-white">Server Connection</h3>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            Connect to your Proxmox server by providing the server address
          </p>
        </div>
        
        <div className="p-4 border border-gray-200 dark:border-gray-700 rounded-lg dark:bg-gray-800">
          <div className="flex justify-center mb-4">
            <FaKey className="text-4xl text-primary-500" />
          </div>
          <h3 className="font-medium mb-2 dark:text-white">API Token Setup</h3>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            Create and configure an API token with the right permissions
          </p>
        </div>
        
        <div className="p-4 border border-gray-200 dark:border-gray-700 rounded-lg dark:bg-gray-800">
          <div className="flex justify-center mb-4">
            <FaChartBar className="text-4xl text-primary-500" />
          </div>
          <h3 className="font-medium mb-2 dark:text-white">Start Monitoring</h3>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            Begin monitoring your VMs and containers in real-time
          </p>
        </div>
      </div>
      
      <p className="text-gray-600 dark:text-gray-300 mb-8">
        You'll need admin access to your Proxmox server to create the required API token.
      </p>
      
      <button onClick={nextStep} className="btn btn-primary w-full">
        Let's Get Started
      </button>
    </div>
  );
};

export default WelcomeStep; 