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
    {
      pattern: /^dark:(bg|text)-(red|yellow|green|blue)-(100|200|300|400|500|600|700|800|900)(\/50)?$/,
    },
    'vm-icon',
    'ct-icon',
  ],
  theme: {
    extend: {},
  },
  plugins: [],
}

