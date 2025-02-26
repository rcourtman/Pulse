import React from 'react';
import { FaCheckCircle, FaServer, FaPlus } from 'react-icons/fa';

const CompletionStep = ({ serverData, completeOnboarding }) => {
  return (
    <div className="text-center">
      <div className="flex justify-center mb-6">
        <div className="rounded-full bg-green-100 p-3">
          <FaCheckCircle className="text-4xl text-green-500" />
        </div>
      </div>
      
      <h2 className="text-2xl font-semibold mb-4">Setup Complete!</h2>
      <p className="text-gray-600 mb-8">
        Your Proxmox server has been successfully configured. You can now start monitoring your VMs and containers.
      </p>
      
      <div className="bg-gray-50 p-4 rounded-md mb-8 text-left">
        <h3 className="font-medium mb-3">Server Details</h3>
        <div className="grid grid-cols-2 gap-2 text-sm">
          <div className="text-gray-500">Name:</div>
          <div className="font-medium">{serverData.name}</div>
          
          <div className="text-gray-500">URL:</div>
          <div className="font-medium">{serverData.url}</div>
          
          <div className="text-gray-500">Token ID:</div>
          <div className="font-medium">{serverData.tokenId}</div>
          
          <div className="text-gray-500">Token:</div>
          <div className="font-medium">••••••••••••••••</div>
        </div>
      </div>
      
      <div className="flex flex-col space-y-3">
        <button
          onClick={completeOnboarding}
          className="btn btn-primary flex items-center justify-center"
        >
          <FaServer className="mr-2" />
          Go to Dashboard
        </button>
        
        <p className="text-sm text-gray-500">
          You can add more servers later from the Servers page.
        </p>
      </div>
    </div>
  );
};

export default CompletionStep; 