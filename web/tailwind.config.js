/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    extend: {
      colors: {
        // Ink palette is driven by CSS variables defined in style.css so
        // every bg-ink-* / text-ink-* class auto-swaps between light & dark.
        // In light: ink-50 is the page bg, ink-900 is primary text.
        // In dark: ink-50 is the page bg (near-black), ink-900 is white.
        ink: {
          50:  'rgb(var(--ink-50)  / <alpha-value>)',
          100: 'rgb(var(--ink-100) / <alpha-value>)',
          200: 'rgb(var(--ink-200) / <alpha-value>)',
          300: 'rgb(var(--ink-300) / <alpha-value>)',
          400: 'rgb(var(--ink-400) / <alpha-value>)',
          500: 'rgb(var(--ink-500) / <alpha-value>)',
          600: 'rgb(var(--ink-600) / <alpha-value>)',
          700: 'rgb(var(--ink-700) / <alpha-value>)',
          800: 'rgb(var(--ink-800) / <alpha-value>)',
          900: 'rgb(var(--ink-900) / <alpha-value>)',
          950: 'rgb(var(--ink-950) / <alpha-value>)',
        },
        // Card / raised surface. White in light, near-black in dark.
        surface: {
          DEFAULT: 'rgb(var(--surface)        / <alpha-value>)',
          raised:  'rgb(var(--surface-raised) / <alpha-value>)',
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
