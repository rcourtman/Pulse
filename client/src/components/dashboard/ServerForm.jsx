import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { FaSpinner } from 'react-icons/fa';

const ServerForm = ({ initialData = {}, onSubmit, onCancel }) => {
  const [isValidating, setIsValidating] = useState(false);
  const isEditing = !!initialData.id;
  
  const { register, handleSubmit, formState: { errors } } = useForm({
    defaultValues: {
      name: initialData.name || '',
      url: initialData.url || '',
      tokenId: initialData.tokenId || '',
      token: '', // Don't pre-fill token for security reasons
    },
  });
  
  const handleFormSubmit = async (data) => {
    setIsValidating(true);
    
    try {
      // In a real app, we would validate the server connection here
      await new Promise(resolve => setTimeout(resolve, 1000));
      
      // If editing and token is empty, use the existing token
      if (isEditing && !data.token) {
        onSubmit({
          name: data.name,
          url: data.url,
          tokenId: data.tokenId,
        });
      } else {
        onSubmit(data);
      }
    } finally {
      setIsValidating(false);
    }
  };
  
  return (
    <form onSubmit={handleSubmit(handleFormSubmit)}>
      <div className="grid grid-cols-1 gap-4 mb-6">
        <div>
          <label htmlFor="name" className="block text-sm font-medium text-gray-700 mb-1">
            Server Name
          </label>
          <input
            id="name"
            type="text"
            className={`input w-full ${errors.name ? 'border-red-500' : ''}`}
            placeholder="My Proxmox Server"
            {...register('name', { required: 'Server name is required' })}
          />
          {errors.name && (
            <p className="mt-1 text-sm text-red-600">{errors.name.message}</p>
          )}
        </div>
        
        <div>
          <label htmlFor="url" className="block text-sm font-medium text-gray-700 mb-1">
            Server URL
          </label>
          <input
            id="url"
            type="text"
            className={`input w-full ${errors.url ? 'border-red-500' : ''}`}
            placeholder="https://proxmox.example.com:8006"
            {...register('url', {
              required: 'Server URL is required',
              pattern: {
                value: /^https?:\/\/.+/,
                message: 'Please enter a valid URL starting with http:// or https://',
              },
            })}
          />
          {errors.url && (
            <p className="mt-1 text-sm text-red-600">{errors.url.message}</p>
          )}
        </div>
        
        <div>
          <label htmlFor="tokenId" className="block text-sm font-medium text-gray-700 mb-1">
            Token ID
          </label>
          <input
            id="tokenId"
            type="text"
            className={`input w-full ${errors.tokenId ? 'border-red-500' : ''}`}
            placeholder="user@pam!token-name"
            {...register('tokenId', { required: 'Token ID is required' })}
          />
          {errors.tokenId && (
            <p className="mt-1 text-sm text-red-600">{errors.tokenId.message}</p>
          )}
        </div>
        
        <div>
          <label htmlFor="token" className="block text-sm font-medium text-gray-700 mb-1">
            API Token {isEditing && <span className="text-gray-500 text-xs">(Leave blank to keep current token)</span>}
          </label>
          <input
            id="token"
            type="password"
            className={`input w-full ${errors.token ? 'border-red-500' : ''}`}
            placeholder={isEditing ? '••••••••••••••••' : 'Enter API token'}
            {...register('token', { 
              required: isEditing ? false : 'API Token is required' 
            })}
          />
          {errors.token && (
            <p className="mt-1 text-sm text-red-600">{errors.token.message}</p>
          )}
        </div>
      </div>
      
      <div className="flex justify-end space-x-3">
        <button
          type="button"
          onClick={onCancel}
          className="btn btn-outline"
        >
          Cancel
        </button>
        <button
          type="submit"
          className="btn btn-primary"
          disabled={isValidating}
        >
          {isValidating ? (
            <>
              <FaSpinner className="animate-spin mr-2" />
              {isEditing ? 'Updating...' : 'Adding...'}
            </>
          ) : (
            isEditing ? 'Update Server' : 'Add Server'
          )}
        </button>
      </div>
    </form>
  );
};

export default ServerForm; 