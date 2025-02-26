import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { FaExclamationTriangle } from 'react-icons/fa';

const ServerDetailsStep = ({ serverData, updateServerData, nextStep, prevStep }) => {
  const [isValidating, setIsValidating] = useState(false);
  const [validationError, setValidationError] = useState(null);
  
  const { register, handleSubmit, formState: { errors } } = useForm({
    defaultValues: {
      name: serverData.name,
      url: serverData.url,
    },
  });
  
  // Validate server connection
  const validateServerConnection = async (data) => {
    setIsValidating(true);
    setValidationError(null);
    
    try {
      // In a real app, we would make an API call to validate the server connection
      // For now, we'll simulate a successful connection after a short delay
      await new Promise(resolve => setTimeout(resolve, 1000));
      
      // Update server data and proceed to next step
      updateServerData({
        name: data.name,
        url: data.url,
      });
      
      nextStep();
    } catch (error) {
      setValidationError('Failed to connect to the server. Please check the URL and try again.');
    } finally {
      setIsValidating(false);
    }
  };
  
  return (
    <div>
      <h2 className="text-2xl font-semibold mb-4">Server Details</h2>
      <p className="text-gray-600 mb-6">
        Enter the details of your Proxmox server.
      </p>
      
      <form onSubmit={handleSubmit(validateServerConnection)}>
        <div className="mb-4">
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
        
        <div className="mb-6">
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
          <p className="mt-1 text-xs text-gray-500">
            Example: https://proxmox.example.com:8006
          </p>
        </div>
        
        {validationError && (
          <div className="mb-6 p-3 bg-red-50 border border-red-200 rounded-md flex items-start">
            <FaExclamationTriangle className="text-red-500 mt-0.5 mr-2 flex-shrink-0" />
            <p className="text-sm text-red-600">{validationError}</p>
          </div>
        )}
        
        <div className="flex justify-between">
          <button
            type="button"
            onClick={prevStep}
            className="btn btn-outline"
          >
            Back
          </button>
          <button
            type="submit"
            className="btn btn-primary"
            disabled={isValidating}
          >
            {isValidating ? 'Validating...' : 'Next'}
          </button>
        </div>
      </form>
    </div>
  );
};

export default ServerDetailsStep; 