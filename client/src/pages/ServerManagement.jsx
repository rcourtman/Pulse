import { useState } from 'react';
import { useServersStore } from '../contexts/ServersStore';
import { FaServer, FaPlus, FaEdit, FaTrash, FaExclamationTriangle } from 'react-icons/fa';
import ServerForm from '../components/dashboard/ServerForm';

const ServerManagement = () => {
  const { servers, addServer, updateServer, removeServer } = useServersStore();
  const [isAddingServer, setIsAddingServer] = useState(false);
  const [editingServer, setEditingServer] = useState(null);
  const [deletingServer, setDeletingServer] = useState(null);
  
  // Start adding a new server
  const startAddServer = () => {
    setIsAddingServer(true);
    setEditingServer(null);
    setDeletingServer(null);
  };
  
  // Start editing a server
  const startEditServer = (server) => {
    setEditingServer(server);
    setIsAddingServer(false);
    setDeletingServer(null);
  };
  
  // Start deleting a server
  const startDeleteServer = (server) => {
    setDeletingServer(server);
    setIsAddingServer(false);
    setEditingServer(null);
  };
  
  // Cancel current action
  const cancelAction = () => {
    setIsAddingServer(false);
    setEditingServer(null);
    setDeletingServer(null);
  };
  
  // Handle server form submission
  const handleServerSubmit = (serverData) => {
    if (editingServer) {
      updateServer(editingServer.id, serverData);
      setEditingServer(null);
    } else {
      addServer({
        id: crypto.randomUUID(),
        ...serverData,
      });
      setIsAddingServer(false);
    }
  };
  
  // Confirm server deletion
  const confirmDeleteServer = () => {
    if (deletingServer) {
      removeServer(deletingServer.id);
      setDeletingServer(null);
    }
  };
  
  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Manage Servers</h1>
        
        {!isAddingServer && !editingServer && !deletingServer && (
          <button
            onClick={startAddServer}
            className="btn btn-primary flex items-center"
          >
            <FaPlus className="mr-2" />
            Add Server
          </button>
        )}
      </div>
      
      {/* Add/Edit Server Form */}
      {(isAddingServer || editingServer) && (
        <div className="bg-white rounded-lg shadow-md p-6 mb-6">
          <h2 className="text-xl font-semibold mb-4">
            {isAddingServer ? 'Add New Server' : 'Edit Server'}
          </h2>
          <ServerForm
            initialData={editingServer || {}}
            onSubmit={handleServerSubmit}
            onCancel={cancelAction}
          />
        </div>
      )}
      
      {/* Delete Confirmation */}
      {deletingServer && (
        <div className="bg-white rounded-lg shadow-md p-6 mb-6">
          <div className="flex items-start">
            <FaExclamationTriangle className="text-red-500 mt-1 mr-3 text-xl flex-shrink-0" />
            <div>
              <h2 className="text-xl font-semibold mb-2">Delete Server</h2>
              <p className="text-gray-600 mb-4">
                Are you sure you want to delete the server "{deletingServer.name}"? This action cannot be undone.
              </p>
              <div className="flex space-x-3">
                <button
                  onClick={confirmDeleteServer}
                  className="btn bg-red-600 text-white hover:bg-red-700"
                >
                  Delete
                </button>
                <button
                  onClick={cancelAction}
                  className="btn btn-outline"
                >
                  Cancel
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
      
      {/* Servers List */}
      {servers.length > 0 ? (
        <div className="bg-white rounded-lg shadow-md overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Server
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  URL
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Token ID
                </th>
                <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {servers.map((server) => (
                <tr key={server.id}>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="flex items-center">
                      <FaServer className="text-gray-400 mr-3" />
                      <div className="font-medium text-gray-900">{server.name}</div>
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {server.url}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {server.tokenId}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                    <button
                      onClick={() => startEditServer(server)}
                      className="text-primary-600 hover:text-primary-900 mr-3"
                    >
                      <FaEdit />
                    </button>
                    <button
                      onClick={() => startDeleteServer(server)}
                      className="text-red-600 hover:text-red-900"
                    >
                      <FaTrash />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="bg-white dark:bg-dark-card rounded-lg shadow-md p-8 text-center">
          <div className="flex justify-center mb-4">
            <FaServer className="text-4xl text-gray-300 dark:text-gray-600" />
          </div>
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">No Servers Added</h3>
          <p className="text-gray-500 dark:text-gray-400 mb-4">
            You haven't added any Proxmox servers yet. Add a server to start monitoring.
          </p>
          <button
            onClick={startAddServer}
            className="btn btn-primary inline-flex items-center"
          >
            <FaPlus className="mr-2" />
            Add Your First Server
          </button>
        </div>
      )}
    </div>
  );
};

export default ServerManagement; 