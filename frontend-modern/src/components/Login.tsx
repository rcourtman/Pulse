import { Component, createSignal, Show } from 'solid-js';
import { setBasicAuth } from '@/utils/apiClient';

interface LoginProps {
  onLogin: () => void;
}

export const Login: Component<LoginProps> = (props) => {
  const [username, setUsername] = createSignal('');
  const [password, setPassword] = createSignal('');
  const [error, setError] = createSignal('');
  const [loading, setLoading] = createSignal(false);

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      // Test the credentials directly first
      const response = await fetch('/api/state', {
        headers: {
          'Authorization': `Basic ${btoa(`${username()}:${password()}`)}`,
          'X-Requested-With': 'XMLHttpRequest', // Prevent browser auth popup
          'Accept': 'application/json'
        },
        credentials: 'include' // Important for session cookie
      });

      if (response.ok) {
        // Credentials are valid, save them and notify parent
        setBasicAuth(username(), password());
        props.onLogin();
      } else if (response.status === 401) {
        setError('Invalid username or password');
        // Clear the input fields
        setUsername('');
        setPassword('');
      } else {
        setError('Server error. Please try again.');
      }
    } catch (err) {
      setError('Failed to connect to server');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div class="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900 py-12 px-4 sm:px-6 lg:px-8">
      <div class="max-w-md w-full space-y-8">
        <div>
          <div class="flex justify-center mb-6">
            <img 
              src="/logo.svg" 
              alt="Pulse Logo" 
              class="w-20 h-20"
            />
          </div>
          <h2 class="text-center text-2xl font-bold text-gray-900 dark:text-white">
            Sign in to Pulse
          </h2>
          <p class="mt-2 text-center text-sm text-gray-600 dark:text-gray-400">
            Enter your credentials to access the dashboard
          </p>
        </div>
        <form class="mt-8 space-y-6 bg-white dark:bg-gray-800 rounded-lg p-8 shadow-sm border border-gray-200 dark:border-gray-700" onSubmit={handleSubmit}>
          <input type="hidden" name="remember" value="true" />
          <div class="space-y-4">
            <div class="relative">
              <label for="username" class="sr-only">
                Username
              </label>
              <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <svg class="h-5 w-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                </svg>
              </div>
              <input
                id="username"
                name="username"
                type="text"
                autocomplete="username"
                required
                class="appearance-none relative block w-full pl-10 pr-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 sm:text-sm dark:bg-gray-700 dark:border-gray-600 dark:text-white dark:placeholder-gray-400"
                placeholder="Username"
                value={username()}
                onInput={(e) => setUsername(e.currentTarget.value)}
              />
            </div>
            <div class="relative">
              <label for="password" class="sr-only">
                Password
              </label>
              <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <svg class="h-5 w-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                </svg>
              </div>
              <input
                id="password"
                name="password"
                type="password"
                autocomplete="current-password"
                required
                class="appearance-none relative block w-full pl-10 pr-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 sm:text-sm dark:bg-gray-700 dark:border-gray-600 dark:text-white dark:placeholder-gray-400"
                placeholder="Password"
                value={password()}
                onInput={(e) => setPassword(e.currentTarget.value)}
              />
            </div>
          </div>

          <Show when={error()}>
            <div class="rounded-md bg-red-50 dark:bg-red-900/20 p-4">
              <div class="flex">
                <div class="flex-shrink-0">
                  <svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
                  </svg>
                </div>
                <div class="ml-3">
                  <p class="text-sm text-red-800 dark:text-red-200">{error()}</p>
                </div>
              </div>
            </div>
          </Show>

          <div>
            <button
              type="submit"
              disabled={loading()}
              class="w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              <Show when={loading()}>
                <svg class="animate-spin -ml-1 mr-3 h-5 w-5 text-white" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
              </Show>
              <Show when={loading()} fallback="Sign in">
                Signing in...
              </Show>
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};