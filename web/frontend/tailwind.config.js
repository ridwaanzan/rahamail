/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    '../views/**/*.tmpl',
    './src/**/*.{js,html}'
  ],
  theme: {
    extend: {
      colors: {
        // Dark Mode Colors matching mockup (Midnight navy / Deep slate)
        dark: {
          bg: '#0b1320',          // Body background
          sidebar: '#0e1726',     // Sidebar background
          header: '#111c30',      // Header / Topbar
          card: '#131e33',        // Card & Panel background
          cardHover: '#182640',   // Card hover
          border: '#1e2c47',      // Subtle border
          borderLight: '#283958', // Highlighted border
          text: '#f8fafc',        // White text
          muted: '#94a3b8',       // Muted secondary text
          accent: '#06b6d4',      // Teal / Cyan glow accent
          accentHover: '#0891b2',
          badge: '#17253d'
        },
        // Light Mode Colors matching mockup (Crisp enterprise minimalist)
        light: {
          bg: '#f8fafc',          // Body background
          sidebar: '#ffffff',     // Sidebar background
          header: '#ffffff',      // Header / Topbar
          card: '#ffffff',        // Card background
          cardHover: '#f1f5f9',   // Card hover
          border: '#e2e8f0',      // Light border
          borderLight: '#cbd5e1',
          text: '#0f172a',        // Dark text
          muted: '#64748b',       // Muted text
          accent: '#0284c7',      // Sky / Royal blue
          accentHover: '#0369a1',
          badge: '#f1f5f9'
        }
      },
      fontFamily: {
        sans: ['system-ui', '-apple-system', 'BlinkMacSystemFont', '"Segoe UI"', 'Roboto', 'sans-serif'],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace']
      },
      boxShadow: {
        'card-dark': '0 4px 20px -2px rgba(0, 0, 0, 0.4)',
        'card-light': '0 1px 3px 0 rgba(0, 0, 0, 0.05), 0 1px 2px -1px rgba(0, 0, 0, 0.05)',
        'cyan-glow': '0 0 15px rgba(6, 182, 212, 0.35)',
        'blue-glow': '0 0 15px rgba(2, 132, 199, 0.35)'
      }
    }
  },
  plugins: []
}
