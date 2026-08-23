/* Shared Tailwind theme — darkstyle (canvas + iris + mint) */
tailwind.config = {
  theme: {
    extend: {
      colors: {
        ink: {
          DEFAULT: '#EDEEF4',
          600: '#A8ABBE',
          700: '#C8CADE',
        },
        muted: '#8D92A6',
        line: '#242836',
        paper: '#0A0B0F',
        canvas: '#0A0B0F',
        surface: '#12141C',
        edge: '#242836',
        edge2: '#2F3546',
        iris: '#7B6CF6',
        iris2: '#9A8FFF',
        mint: '#3DDC97',
        sun: {
          100: 'rgba(61,220,151,0.12)',
          200: 'rgba(61,220,151,0.22)',
          300: '#3DDC97',
          400: '#9A8FFF',
          500: '#7B6CF6',
        },
        up: '#3DDC97',
        down: '#FF5F73',
        accent: '#7B6CF6',
      },
      fontFamily: {
        sans: ['"Inter"', 'Segoe UI', 'Roboto', 'system-ui', 'sans-serif'],
        display: ['"Plus Jakarta Sans"', 'Segoe UI', 'Roboto', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'SF Mono', 'Roboto Mono', 'ui-monospace', 'Menlo', 'monospace'],
      },
    },
  },
};
