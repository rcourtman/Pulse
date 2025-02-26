import { useState, useEffect } from 'react';

// Get the API base URL from environment or use a default
const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:3000';

/**
 * Hook to fetch pre-configured Proxmox nodes from the server
 */
const useConfiguredNodes = () => {
  const [nodes, setNodes] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState(null);
  
  useEffect(() => {
    const fetchNodes = async () => {
      try {
        console.log(`Fetching configured nodes from ${API_BASE_URL}/api/nodes`);
        setIsLoading(true);
        setError(null);
        
        const response = await fetch(`${API_BASE_URL}/api/nodes`);
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`Failed to fetch nodes: ${response.status} ${response.statusText}`, errorText);
          throw new Error(`Failed to fetch nodes: ${response.status} ${response.statusText}`);
        }
        
        const data = await response.json();
        console.log('Received nodes data:', data);
        
        if (Array.isArray(data)) {
          // Transform the nodes data to match the expected format
          const formattedNodes = data.map(node => {
            // Ensure node ID is properly formatted
            const nodeId = node.id ? 
              (node.id.startsWith('node-') ? node.id : `node-${node.id}`) : 
              `node-${Math.random().toString(36).substring(2, 9)}`;
              
            return {
              id: nodeId,
              name: node.name || `Node ${node.id || 'Unknown'}`,
              url: node.url,
              status: 'unknown',
              type: 'proxmox',
            };
          });
          
          console.log('Formatted nodes:', formattedNodes);
          setNodes(formattedNodes);
        } else if (data && data.data && Array.isArray(data.data)) {
          // Handle legacy API format
          console.log('Received legacy format nodes data:', data);
          const formattedNodes = data.data.map(node => ({
            id: node.id,
            name: node.name || `Node ${node.id}`,
            url: node.url,
            status: 'unknown',
            type: 'proxmox',
          }));
          
          console.log('Formatted legacy nodes:', formattedNodes);
          setNodes(formattedNodes);
        } else {
          console.error('Invalid nodes data format:', data);
          throw new Error('Invalid response format from server');
        }
      } catch (err) {
        console.error('Error fetching nodes:', err);
        setError(err.message || 'Failed to load configured nodes');
        // Return empty array on error
        setNodes([]);
      } finally {
        setIsLoading(false);
      }
    };
    
    fetchNodes();
  }, []);
  
  return { nodes, isLoading, error };
};

export default useConfiguredNodes; 