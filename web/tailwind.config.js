/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        brand: {
          primary: '#f97316',
          surface: '#0e1725',
        },
      },
    },
  },
  plugins: [],
  corePlugins: {
    preflight: false,
  },
}
