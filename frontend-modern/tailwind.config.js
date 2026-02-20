import typography from '@tailwindcss/typography';

export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    screens: {
      'xs': '400px',
      'sm': '640px',
      'md': '768px',
      'lg': '1024px',
      'xl': '1280px',
      '2xl': '1536px',
    },
    extend: {
      colors: {
        gray: {
          750: '#2d3748',
        },
        red: { 900: 'rgba(127, 29, 29, 0.25)', 950: 'rgba(69, 10, 10, 0.25)' },
        amber: { 900: 'rgba(120, 53, 15, 0.25)', 950: 'rgba(67, 20, 7, 0.25)' },
        yellow: { 900: 'rgba(113, 63, 18, 0.25)', 950: 'rgba(66, 32, 6, 0.25)' },
        green: { 900: 'rgba(20, 83, 45, 0.25)', 950: 'rgba(5, 46, 22, 0.25)' },
        emerald: { 900: 'rgba(6, 78, 59, 0.25)', 950: 'rgba(2, 44, 34, 0.25)' },
        teal: { 900: 'rgba(19, 78, 74, 0.25)', 950: 'rgba(4, 47, 46, 0.25)' },
        cyan: { 900: 'rgba(22, 78, 99, 0.25)', 950: 'rgba(8, 51, 68, 0.25)' },
        sky: { 900: 'rgba(12, 74, 110, 0.25)', 950: 'rgba(8, 47, 73, 0.25)' },
        blue: { 900: 'rgba(30, 58, 138, 0.25)', 950: 'rgba(23, 37, 84, 0.25)' },
        indigo: { 900: 'rgba(49, 46, 129, 0.25)', 950: 'rgba(30, 27, 75, 0.25)' },
        purple: { 900: 'rgba(88, 28, 135, 0.25)', 950: 'rgba(59, 7, 100, 0.25)' },
        rose: { 900: 'rgba(136, 19, 55, 0.25)', 950: 'rgba(76, 5, 25, 0.25)' },
      },
      animation: {
        'spin-slow': 'spin 2s linear infinite',
        'fadeIn': 'fadeIn 0.2s ease-in',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        }
      }
    },
  },
  plugins: [typography],
}