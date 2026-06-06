/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    extend: {
      colors: {
        ink: {
          50:  '#f7f7f8',
          100: '#eeeef0',
          200: '#dcdde1',
          300: '#c2c4ca',
          400: '#9b9ea7',
          500: '#737680',
          600: '#54565e',
          700: '#3a3b41',
          800: '#212226',
          900: '#131418',
          950: '#0a0a0c',
        },
        accent: { DEFAULT: '#0a0a0c', muted: '#3a3b41' },
        success: { DEFAULT: '#16a34a', soft: '#dcfce7' },
        warning: { DEFAULT: '#d97706', soft: '#fef3c7' },
        danger:  { DEFAULT: '#dc2626', soft: '#fee2e2' },
      },
      borderRadius: { lg: '10px', xl: '14px', '2xl': '18px' },
      fontFamily: {
        sans: ['ui-sans-serif', 'system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'Inter', 'sans-serif'],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace'],
      },
      boxShadow: {
        card: '0 1px 0 rgba(10,10,12,0.04), 0 1px 2px rgba(10,10,12,0.04)',
        pop:  '0 12px 32px -8px rgba(10,10,12,0.18), 0 4px 8px -4px rgba(10,10,12,0.08)',
      },
    },
  },
  plugins: [],
}
