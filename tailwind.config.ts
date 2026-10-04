import type { Config } from "tailwindcss";

export default {
  content: ["./app/**/*.{ts,tsx}"],
  future: { hoverOnlyWhenSupported: true },
  theme: {
    extend: {
      colors: {
        canvas: "#FAFAFA",
        paper: "#FFFFFF",
        ink: "#232323",
        muted: "#666666",
        faint: "#737373",
        line: "#E5E5E5",
        accent: "#EA580C",
        "accent-dark": "#C2410C",
        "accent-soft": "#F8C5AA",
      },
      fontFamily: {
        sans: ["Inter", "-apple-system", "BlinkMacSystemFont", "Segoe UI", "sans-serif"],
        mono: ["JetBrains Mono", "SFMono-Regular", "Consolas", "monospace"],
        display: ["Geist Mono", "JetBrains Mono", "monospace"],
      },
      transitionTimingFunction: {
        out: "cubic-bezier(0.23, 1, 0.32, 1)",
      },
      keyframes: {
        rise: {
          from: { opacity: "0", transform: "translateY(8px)" },
          to: { opacity: "1", transform: "translateY(0)" },
        },
        highlight: {
          from: { clipPath: "inset(0 100% 0 0)" },
          to: { clipPath: "inset(0 0 0 0)" },
        },
      },
      animation: {
        rise: "rise 500ms cubic-bezier(0.23, 1, 0.32, 1) both",
        highlight: "highlight 700ms cubic-bezier(0.77, 0, 0.175, 1) 550ms both",
      },
    },
  },
  plugins: [],
} satisfies Config;
