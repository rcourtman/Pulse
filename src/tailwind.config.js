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

