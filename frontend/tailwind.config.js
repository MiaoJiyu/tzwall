/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        ink: {
          DEFAULT: '#1F1A22',
          soft: '#6B6672',
        },
        paper: {
          DEFAULT: '#FBF9F7',
          white: '#FFFFFF',
        },
        grape: {
          50: '#F3EFF4',
          100: '#E9D8EC',
          300: '#A85CB0',
          500: '#8A3D93',
          600: '#6B2A73',
          700: '#521759',
        },
        jade: '#2E7D5B',
        warn: '#C9A227',
        danger: '#C0392B',
      },
      fontFamily: {
        serif: ['"Noto Serif SC"', 'Songti SC', 'STSong', 'serif'],
        sans: ['"PingFang SC"', '"Noto Sans SC"', 'system-ui', 'sans-serif'],
      },
      boxShadow: {
        card: '0 1px 2px rgba(82,23,89,0.04), 0 8px 24px rgba(82,23,89,0.06)',
        soft: '0 2px 12px rgba(82,23,89,0.08)',
      },
      borderRadius: {
        xl2: '16px',
      },
      keyframes: {
        floaty: {
          '0%,100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-4px)' },
        },
        fadeup: {
          '0%': { opacity: '0', transform: 'translateY(8px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
      },
      animation: {
        floaty: 'floaty 4s ease-in-out infinite',
        fadeup: 'fadeup 0.4s ease-out both',
      },
    },
  },
  plugins: [require('tailwindcss-animate')],
}
