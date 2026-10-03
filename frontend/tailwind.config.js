/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: { ink: '#17211d', forest: '#0d3327', mint: '#dff4e9', canvas: '#f5f7f4' },
      boxShadow: { card: '0 1px 2px rgb(13 51 39 / 0.05), 0 8px 24px rgb(13 51 39 / 0.06)' },
      fontFamily: { sans: ['Inter', 'ui-sans-serif', 'system-ui', 'sans-serif'] }
    }
  },
  plugins: []
}

