import React, { createContext, useContext, useReducer, useEffect } from 'react';

interface Node {
  id: string;
  name?: string;
  host: string;
  tokenId: string;
  status?: 'online' | 'offline' | 'error';
  metrics?: any;
}

interface NodeState {
  nodes: Record<string, Node>;
}

type NodeAction = 
  | { type: 'UPDATE_NODE'; payload: Node }
  | { type: 'REMOVE_NODE'; payload: string };

const nodeReducer = (state: NodeState, action: NodeAction): NodeState => {
  console.log('NodeReducer:', { action, currentState: state }); // Debug log
  
  switch (action.type) {
    case 'UPDATE_NODE':
      const newState = {
        ...state,
        nodes: {
          ...state.nodes,
          [action.payload.id]: action.payload
        }
      };
      console.log('New state after update:', newState); // Debug log
      return newState;
    case 'REMOVE_NODE':
      const { [action.payload]: removed, ...remaining } = state.nodes;
      return {
        ...state,
        nodes: remaining
      };
    default:
      return state;
  }
};

const NodeContext = createContext<{
  state: NodeState;
  dispatch: React.Dispatch<NodeAction>;
} | null>(null);

export const NodeProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(nodeReducer, { nodes: {} });

  console.log('NodeProvider rendering:', state); // Debug log

  // Clear localStorage on mount to ensure fresh start
  useEffect(() => {
    localStorage.removeItem('nodes');
  }, []);

  // Save nodes to localStorage when they change
  useEffect(() => {
    localStorage.setItem('nodes', JSON.stringify(state.nodes));
  }, [state.nodes]);

  return (
    <NodeContext.Provider value={{ state, dispatch }}>
      {children}
    </NodeContext.Provider>
  );
};

export const useNodes = () => {
  const context = useContext(NodeContext);
  if (!context) {
    throw new Error('useNodes must be used within a NodeProvider');
  }
  return context;
}; 