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
    'grid-cols-1', 'grid-cols-2', 'grid-cols-3', 'grid-cols-4',
    'sm:grid-cols-1', 'sm:grid-cols-2', 'sm:grid-cols-3', 'sm:grid-cols-4',
    'md:grid-cols-1', 'md:grid-cols-2', 'md:grid-cols-3', 'md:grid-cols-4',
    'lg:grid-cols-1', 'lg:grid-cols-2', 'lg:grid-cols-3', 'lg:grid-cols-4',
    'xl:grid-cols-1', 'xl:grid-cols-2', 'xl:grid-cols-3', 'xl:grid-cols-4',
    'vm-icon',
    'ct-icon',
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

