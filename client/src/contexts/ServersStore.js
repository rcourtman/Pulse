import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import CryptoJS from 'crypto-js';

// Encryption key - in a real app, this would be more securely managed
const ENCRYPTION_KEY = 'pulse-app-secret-key';

// Helper functions for encryption/decryption
const encrypt = (text) => {
  return CryptoJS.AES.encrypt(text, ENCRYPTION_KEY).toString();
};

const decrypt = (ciphertext) => {
  const bytes = CryptoJS.AES.decrypt(ciphertext, ENCRYPTION_KEY);
  return bytes.toString(CryptoJS.enc.Utf8);
};

// Create the store with persistence
export const useServersStore = create(
  persist(
    (set, get) => ({
      servers: [],
      initialized: false,
      
      // Set initialized state
      setInitialized: (value) => set({ initialized: value }),
      
      // Add a new server
      addServer: (server) => {
        // Encrypt the token
        const encryptedServer = {
          ...server,
          token: encrypt(server.token),
        };
        
        set((state) => ({
          servers: [...state.servers, encryptedServer],
        }));
      },
      
      // Update an existing server
      updateServer: (id, updatedServer) => {
        set((state) => ({
          servers: state.servers.map((server) => {
            if (server.id === id) {
              // If token is changed, encrypt it
              const token = updatedServer.token !== server.token
                ? encrypt(updatedServer.token)
                : server.token;
                
              return { ...server, ...updatedServer, token };
            }
            return server;
          }),
        }));
      },
      
      // Remove a server
      removeServer: (id) => {
        set((state) => ({
          servers: state.servers.filter((server) => server.id !== id),
        }));
      },
      
      // Get a server with decrypted token
      getServerWithDecryptedToken: (id) => {
        const server = get().servers.find((s) => s.id === id);
        if (!server) return null;
        
        return {
          ...server,
          token: decrypt(server.token),
        };
      },
      
      // Get all servers with decrypted tokens
      getAllServersWithDecryptedTokens: () => {
        return get().servers.map((server) => ({
          ...server,
          token: decrypt(server.token),
        }));
      },
    }),
    {
      name: 'pulse-servers-storage',
      // Only persist the servers array
      partialize: (state) => ({ servers: state.servers }),
    }
  )
); 