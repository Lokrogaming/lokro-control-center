/** @type {import('tailwindcss').Config} */
export default {
  darkMode: "class",
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        lokro: {
          dark: "#0b0f13",
          panel: "#121921",
          border: "#1f2a33",
          cyan: "#22d3ee",
          green: "#4ade80",
          muted: "#64748b",
        },
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "sans-serif"],
        mono: ["ui-monospace", "SFMono-Regular", "monospace"],
      },
      boxShadow: {
        "lokro-glow": "0 0 20px -10px rgb(34 211 238 / 0.35)",
        "lokro-glow-green": "0 0 20px -10px rgb(74 222 128 / 0.25)",
      },
      animation: {
        "pulse-slow": "pulse 4s cubic-bezier(0.4, 0, 0.6, 1) infinite",
      },
    },
  },
  plugins: [],
}
