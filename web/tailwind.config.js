/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    extend: {
      colors: {
        // Ink palette via CSS variables (style.css). Auto-inverts per theme.
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
        surface: {
          DEFAULT: 'rgb(var(--surface)        / <alpha-value>)',
          raised:  'rgb(var(--surface-raised) / <alpha-value>)',
        },
        success: { DEFAULT: '#34c759' },
        warning: { DEFAULT: '#ff9500' },
        danger:  { DEFAULT: '#ff3b30' },
        // Phosphor amber — live/active accent. Warm gold, works on both themes.
        amber: {
          DEFAULT: '#E8A041',
          50:  '#FDF5E8',
          100: '#FAE8C8',
          200: '#F5D08A',
          300: '#EEB84D',
          400: '#E8A041',
          500: '#D4882A',
          600: '#B87020',
          700: '#8C5418',
          800: '#613A10',
          900: '#3D2408',
        },
      },
      borderRadius: { lg: '10px', xl: '14px', '2xl': '18px', '3xl': '22px', '4xl': '28px' },
      fontFamily: {
        // One sans for everything — Onest, variable, with Cyrillic.
        sans: ['"Onest"', 'ui-sans-serif', 'system-ui', '-apple-system', 'sans-serif'],
        // Mono strictly for IPs, keys, magic params.
        mono: ['"JetBrains Mono"', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'],
      },
      letterSpacing: {
        tightest: '-0.045em',
        chrome:   '-0.011em',
      },
      boxShadow: {
        card:   '0 1px 2px rgba(10,10,12,0.03), 0 4px 16px -8px rgba(10,10,12,0.05)',
        pop:    '0 18px 48px -12px rgba(10,10,12,0.18), 0 6px 12px -6px rgba(10,10,12,0.07)',
        lifted: '0 2px 4px rgba(10,10,12,0.04), 0 12px 32px -12px rgba(10,10,12,0.12)',
      },
      keyframes: {
        rise: {
          '0%':   { opacity: '0', transform: 'translateY(8px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        'rise-sm': {
          '0%':   { opacity: '0', transform: 'translateY(4px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        pulse: {
          '0%, 100%': { transform: 'scaleX(0.18)', opacity: '0.45' },
          '50%':      { transform: 'scaleX(1)',    opacity: '1'    },
        },
        wipe: {
          '0%':   { transform: 'scaleX(0)', transformOrigin: '0% 50%' },
          '100%': { transform: 'scaleX(1)', transformOrigin: '0% 50%' },
        },
        'ping-slow': {
          '0%':    { transform: 'scale(1)', opacity: '0.6' },
          '100%':  { transform: 'scale(2)', opacity: '0'   },
        },
        'fade-in': {
          '0%':   { opacity: '0' },
          '100%': { opacity: '1' },
        },
      },
      animation: {
        rise:       'rise 0.45s cubic-bezier(0.2, 0.8, 0.2, 1) both',
        'rise-sm':  'rise-sm 0.35s cubic-bezier(0.2, 0.8, 0.2, 1) both',
        pulse:      'pulse 2.4s ease-in-out infinite',
        wipe:       'wipe 0.9s cubic-bezier(0.2, 0.8, 0.2, 1) both',
        'ping-slow':'ping-slow 2s cubic-bezier(0, 0, 0.2, 1) infinite',
        'fade-in':  'fade-in 0.3s ease both',
      },
    },
  },
  plugins: [],
}
