import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { FaCheckCircle, FaExclamationTriangle } from 'react-icons/fa';

const TokenVerificationStep = ({ serverData, updateServerData, nextStep, prevStep }) => {
  const [isVerifying, setIsVerifying] = useState(false);
  const [verificationStatus, setVerificationStatus] = useState(null);
  
  const { register, handleSubmit, formState: { errors } } = useForm({
    defaultValues: {
      tokenId: serverData.tokenId,
      token: serverData.token,
    },
  });
  
  // Verify token
  const verifyToken = async (data) => {
    setIsVerifying(true);
    setVerificationStatus(null);
    
    try {
      // In a real app, we would make an API call to verify the token
      // For now, we'll simulate a successful verification after a short delay
      await new Promise(resolve => setTimeout(resolve, 1500));
      
      // Update server data with token information
      updateServerData({
        tokenId: data.tokenId,
        token: data.token,
      });
      
      setVerificationStatus({
        success: true,
        message: 'Token verified successfully! You can now proceed.',
      });
    } catch (error) {
      setVerificationStatus({
        success: false,
        message: 'Failed to verify token. Please check your token details and try again.',
      });
    } finally {
      setIsVerifying(false);
    }
  };
  
  return (
    <div>
      <h2 className="text-2xl font-semibold mb-4">Verify API Token</h2>
      <p className="text-gray-600 mb-6">
        Enter the API token details you created in the previous step.
      </p>
      
      <form onSubmit={handleSubmit(verifyToken)}>
        <div className="mb-4">
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
          <p className="mt-1 text-xs text-gray-500">
            Format: user@pam!token-name (e.g., root@pam!pulse-monitor)
          </p>
        </div>
        
        <div className="mb-6">
          <label htmlFor="token" className="block text-sm font-medium text-gray-700 mb-1">
            API Token
          </label>
          <input
            id="token"
            type="password"
            className={`input w-full ${errors.token ? 'border-red-500' : ''}`}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            {...register('token', { required: 'API Token is required' })}
          />
          {errors.token && (
            <p className="mt-1 text-sm text-red-600">{errors.token.message}</p>
          )}
          <p className="mt-1 text-xs text-gray-500">
            This is the secret value you received when creating the token
          </p>
        </div>
        
        {verificationStatus && (
          <div className={`mb-6 p-3 rounded-md flex items-start ${
            verificationStatus.success 
              ? 'bg-green-50 border border-green-200' 
              : 'bg-red-50 border border-red-200'
          }`}>
            {verificationStatus.success ? (
              <FaCheckCircle className="text-green-500 mt-0.5 mr-2 flex-shrink-0" />
            ) : (
              <FaExclamationTriangle className="text-red-500 mt-0.5 mr-2 flex-shrink-0" />
            )}
            <p className={`text-sm ${
              verificationStatus.success ? 'text-green-600' : 'text-red-600'
            }`}>
              {verificationStatus.message}
            </p>
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
          
          <div className="space-x-3">
            <button
              type="submit"
              className="btn btn-secondary"
              disabled={isVerifying}
            >
              {isVerifying ? 'Verifying...' : 'Verify Token'}
            </button>
            
            <button
              type="button"
              onClick={nextStep}
              className="btn btn-primary"
              disabled={!verificationStatus?.success}
            >
              Next
            </button>
          </div>
        </div>
      </form>
    </div>
  );
};

export default TokenVerificationStep; 