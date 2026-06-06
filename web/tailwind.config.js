/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    extend: {
      colors: {
        // iOS-system inspired neutrals. Page bg = ink-50, cards = white,
        // dividers = ink-100, primary text = ink-900.
        ink: {
          50:  '#f2f2f7', // systemGray6
          100: '#e5e5ea', // systemGray5
          200: '#d1d1d6', // systemGray4
          300: '#c7c7cc', // systemGray3
          400: '#aeaeb2', // systemGray2
          500: '#8e8e93', // systemGray
          600: '#6c6c70',
          700: '#48484a',
          800: '#2c2c2e',
          900: '#1c1c1e',
          950: '#0a0a0c',
        },
        accent: { DEFAULT: '#0a0a0c', muted: '#3a3b41' },
        // Avatar palette — used by Avatar atom to color initials.
        avatar: {
          1: '#FF9F0A', 2: '#30B0C7', 3: '#34C759',
          4: '#5856D6', 5: '#FF375F', 6: '#AF52DE',
          7: '#FF9500', 8: '#64D2FF',
        },
        success: { DEFAULT: '#34c759', soft: '#dcfce7' },
        warning: { DEFAULT: '#ff9500', soft: '#fef3c7' },
        danger:  { DEFAULT: '#ff3b30', soft: '#fee2e2' },
      },
      borderRadius: { lg: '10px', xl: '14px', '2xl': '18px', '3xl': '22px' },
      fontFamily: {
        sans: ['ui-sans-serif', 'system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'Inter', 'sans-serif'],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace'],
      },
      boxShadow: {
        card: '0 1px 2px rgba(10,10,12,0.04), 0 4px 16px -8px rgba(10,10,12,0.06)',
        pop:  '0 18px 48px -12px rgba(10,10,12,0.20), 0 6px 12px -6px rgba(10,10,12,0.08)',
        glass: '0 1px 0 rgba(255,255,255,0.7) inset, 0 -1px 0 rgba(10,10,12,0.04)',
      },
      backdropBlur: { glass: '24px' },
      backdropSaturate: { glass: '180%' },
    },
  },
  plugins: [],
}
