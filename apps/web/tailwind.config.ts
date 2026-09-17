import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: "class",
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./lib/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        surface: {
          ground: "var(--surface-ground)",
          card: "var(--surface-card)",
          elevated: "var(--surface-elevated)",
          muted: "var(--surface-muted)",
          hover: "var(--surface-hover)",
          active: "var(--surface-active)",
        },
        text: {
          primary: "var(--text-primary)",
          secondary: "var(--text-secondary)",
          muted: "var(--text-muted)",
        },
        border: {
          subtle: "var(--border-subtle)",
          strong: "var(--border-strong)",
        },
        primary: {
          DEFAULT: "var(--primary)",
          hover: "var(--primary-hover)",
          active: "var(--primary-active)",
          fg: "var(--primary-fg)",
        },
        destructive: {
          DEFAULT: "var(--destructive)",
          hover: "var(--destructive-hover)",
          active: "var(--destructive-active)",
          fg: "var(--destructive-fg)",
        },
        warning: {
          surface: "var(--warning-surface)",
          border: "var(--warning-border)",
          text: "var(--warning-text)",
        },
        info: {
          surface: "var(--info-surface)",
          border: "var(--info-border)",
          text: "var(--info-text)",
        },
        success: {
          surface: "var(--success-surface)",
          border: "var(--success-border)",
          text: "var(--success-text)",
        },
        focus: "var(--focus-ring)",
        ring: {
          focus: "var(--focus-ring)",
        },
      },
    },
  },
  plugins: [],
};

export default config;
