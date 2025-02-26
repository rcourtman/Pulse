import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useServersStore } from '../contexts/ServersStore';
import WelcomeStep from '../components/onboarding/WelcomeStep';
import ServerDetailsStep from '../components/onboarding/ServerDetailsStep';
import TokenCreationStep from '../components/onboarding/TokenCreationStep';
import TokenVerificationStep from '../components/onboarding/TokenVerificationStep';
import CompletionStep from '../components/onboarding/CompletionStep';
import React from 'react';

const Onboarding = () => {
  const navigate = useNavigate();
  const { addServer } = useServersStore();
  const [currentStep, setCurrentStep] = useState(0);
  const [error, setError] = useState(null);
  const [serverData, setServerData] = useState({
    id: crypto.randomUUID(),
    name: '',
    url: '',
    tokenId: '',
    token: '',
  });

  // Steps in the onboarding process
  const steps = [
    { title: 'Welcome', component: WelcomeStep },
    { title: 'Server Details', component: ServerDetailsStep },
    { title: 'Token Creation', component: TokenCreationStep },
    { title: 'Token Verification', component: TokenVerificationStep },
    { title: 'Completion', component: CompletionStep },
  ];

  // Update server data
  const updateServerData = (data) => {
    setServerData((prev) => ({ ...prev, ...data }));
  };

  // Go to next step
  const nextStep = () => {
    if (currentStep < steps.length - 1) {
      setCurrentStep(currentStep + 1);
    }
  };

  // Go to previous step
  const prevStep = () => {
    if (currentStep > 0) {
      setCurrentStep(currentStep - 1);
    }
  };

  // Complete onboarding
  const completeOnboarding = () => {
    addServer(serverData);
    navigate('/');
  };

  // Render current step with error handling
  const CurrentStepComponent = steps[currentStep].component;

  // Log rendering for debugging
  useEffect(() => {
    console.log('Rendering Onboarding component, step:', currentStep);
    console.log('Current step component:', steps[currentStep].title);
  }, [currentStep]);

  // Error handling function
  const handleError = (error) => {
    console.error('Error in onboarding component:', error);
    setError(error);
  };

  // If there's an error, display it
  if (error) {
    return (
      <div className="max-w-2xl mx-auto p-6 bg-red-50 rounded-lg shadow-md">
        <h1 className="text-2xl font-bold text-red-600 mb-4">Error Loading Onboarding</h1>
        <p className="text-red-700 mb-4">{error.message || 'An unknown error occurred'}</p>
        <button 
          className="px-4 py-2 bg-primary-600 text-white rounded hover:bg-primary-700"
          onClick={() => setError(null)}
        >
          Try Again
        </button>
      </div>
    );
  }

  return (
    <div className="max-w-2xl mx-auto">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-center mb-6">
          Set Up Your Proxmox Server
        </h1>
        
        {/* Progress bar */}
        <div className="w-full bg-gray-200 rounded-full h-2.5 mb-6">
          <div
            className="bg-primary-600 h-2.5 rounded-full transition-all duration-300"
            style={{ width: `${((currentStep + 1) / steps.length) * 100}%` }}
          ></div>
        </div>
        
        {/* Step titles */}
        <div className="flex justify-between mb-8">
          {steps.map((step, index) => (
            <div
              key={index}
              className={`text-sm font-medium ${
                index <= currentStep
                  ? 'text-primary-600'
                  : 'text-gray-400'
              }`}
            >
              {step.title}
            </div>
          ))}
        </div>
      </div>

      {/* Current step content */}
      <div className="bg-white rounded-lg shadow-md p-6 mb-6">
        {/* Use error handling with try/catch in a component method */}
        <ErrorBoundary onError={handleError}>
          <CurrentStepComponent
            serverData={serverData}
            updateServerData={updateServerData}
            nextStep={nextStep}
            prevStep={prevStep}
            completeOnboarding={completeOnboarding}
          />
        </ErrorBoundary>
      </div>
    </div>
  );
};

// Simple Error Boundary Component
class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(error) {
    return { hasError: true };
  }

  componentDidCatch(error, errorInfo) {
    console.error('Error caught by boundary:', error, errorInfo);
    if (this.props.onError) {
      this.props.onError(error);
    }
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="text-red-600 p-4">
          Something went wrong. Please try again.
        </div>
      );
    }

    return this.props.children;
  }
}

export default Onboarding; 