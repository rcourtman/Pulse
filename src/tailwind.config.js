/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    './src/public/**/*.html',
    './src/public/**/*.js',
  ],
  safelist: [
    'opacity-60',
    {
      pattern: /^(bg|text)-(red|yellow|green|blue)-(100|200|300|400|500|600|700|800|900)(\/50)?$/,
    },
    // Specific dark mode classes needed for alert management modal
    'dark:bg-blue-900/20',
    'dark:bg-yellow-900/20',
    'dark:border-blue-800',
    'dark:border-yellow-800',
    'dark:text-blue-200',
    'dark:text-blue-300',
    'dark:text-blue-400',
    'dark:text-yellow-200',
    'dark:text-yellow-300',
    'dark:text-yellow-400',
    'dark:hover:text-blue-200',
    'dark:hover:text-yellow-200',
    // Add dark mode backgrounds for sticky columns
    'dark:bg-gray-800',
    'dark:bg-gray-700/80',
    'bg-gray-100',
    'dark:bg-gray-700',
    'dark:text-gray-300',
    'dark:text-gray-400',
    'dark:border-gray-600',
    'dark:border-gray-700',
    'grid-cols-1', 'grid-cols-2', 'grid-cols-3', 'grid-cols-4',
    'sm:grid-cols-1', 'sm:grid-cols-2', 'sm:grid-cols-3', 'sm:grid-cols-4',
    'md:grid-cols-1', 'md:grid-cols-2', 'md:grid-cols-3', 'md:grid-cols-4',
    'lg:grid-cols-1', 'lg:grid-cols-2', 'lg:grid-cols-3', 'lg:grid-cols-4',
    'xl:grid-cols-1', 'xl:grid-cols-2', 'xl:grid-cols-3', 'xl:grid-cols-4',
    'vm-icon',
    'ct-icon',
    'hidden',
    'inline',
    'sm:hidden',
    'sm:inline',
    'truncate',
    'px-1',
    'table-cell',
    'sm:table-cell',
    'md:table-cell',
    'lg:table-cell',
  ],
  theme: {
    extend: {},
    scrollbar: theme => ({
      DEFAULT: {
        size: theme('spacing.3'),
        track: {
          background: theme('colors.gray.100'),
          darkBackground: theme('colors.neutral.700'),
        },
        thumb: {
          background: theme('colors.gray.400'),
          darkBackground: theme('colors.neutral.500'),
          borderRadius: theme('borderRadius.full'),
        },
        hover: {
          background: theme('colors.gray.500'),
          darkBackground: theme('colors.neutral.400'),
        },
      },
    }),
  },
  plugins: [
    require('@gradin/tailwindcss-scrollbar'),
  ],
}

