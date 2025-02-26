import React from 'react';
import { FaKey, FaExternalLinkAlt } from 'react-icons/fa';

const TokenCreationStep = ({ serverData, nextStep, prevStep }) => {
  return (
    <div>
      <h2 className="text-2xl font-semibold mb-4">Create API Token</h2>
      <p className="text-gray-600 mb-6">
        You need to create an API token in your Proxmox server with the appropriate permissions.
      </p>
      
      <div className="bg-gray-50 p-4 rounded-md mb-6">
        <h3 className="font-medium mb-2 flex items-center">
          <FaKey className="mr-2 text-primary-500" />
          Token Creation Steps
        </h3>
        <ol className="list-decimal pl-5 space-y-3 text-sm">
          <li>
            Log in to your Proxmox web interface at{' '}
            <a 
              href={serverData.url} 
              target="_blank" 
              rel="noopener noreferrer"
              className="text-primary-600 hover:underline inline-flex items-center"
            >
              {serverData.url} <FaExternalLinkAlt className="ml-1 text-xs" />
            </a>
          </li>
          <li>Navigate to <strong>Datacenter</strong> → <strong>Permissions</strong> → <strong>API Tokens</strong></li>
          <li>Click on <strong>Add</strong> to create a new API token</li>
          <li>
            Enter the following information:
            <ul className="list-disc pl-5 mt-2 space-y-1">
              <li><strong>User</strong>: A user with appropriate permissions (e.g., root@pam)</li>
              <li><strong>Token ID</strong>: A name for your token (e.g., pulse-monitor)</li>
              <li><strong>Privilege Separation</strong>: Uncheck this option</li>
              <li><strong>Expiration</strong>: (Optional) Set an expiration date if desired</li>
            </ul>
          </li>
          <li>Click <strong>Add</strong> to create the token</li>
          <li>
            <strong className="text-red-600">Important:</strong> Copy both the Token ID and Secret (value) immediately, as the secret will only be shown once
          </li>
        </ol>
      </div>
      
      <div className="bg-yellow-50 p-4 rounded-md border border-yellow-200 mb-6">
        <h3 className="font-medium mb-2 text-yellow-800">Required Permissions</h3>
        <p className="text-sm text-yellow-700 mb-2">
          After creating the token, you need to assign it the following permissions:
        </p>
        <ul className="list-disc pl-5 text-sm text-yellow-700">
          <li>Navigate to <strong>Permissions</strong> → <strong>Add</strong> → <strong>API Token Permission</strong></li>
          <li>Path: <strong>/</strong> (root path)</li>
          <li>Role: <strong>PVEAuditor</strong> (minimum required for monitoring)</li>
          <li>Click <strong>Add</strong> to assign the permission</li>
        </ul>
      </div>
      
      <div className="flex justify-between">
        <button
          type="button"
          onClick={prevStep}
          className="btn btn-outline"
        >
          Back
        </button>
        <button
          type="button"
          onClick={nextStep}
          className="btn btn-primary"
        >
          I've Created the Token
        </button>
      </div>
    </div>
  );
};

export default TokenCreationStep; 