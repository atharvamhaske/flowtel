import type { Config } from "tailwindcss";

export default {
  content: ["./app/**/*.{ts,tsx}"],
  theme: { extend: { colors: { orange: "#ff6b00", paper: "#f3f0ea", ink: "#171717" }, fontFamily: { geist: ["Geist", "Arial", "sans-serif"], instrument: ["Instrument Serif", "Georgia", "serif"] } } },
  plugins: [],
} satisfies Config;
