import type { CSSProperties } from "react";
import type * as T from "@/types/api";
import { isHexColor } from "@/lib/color";

/**
 * Table widgets are recolored through CSS custom properties instead of inline
 * styles per cell, so a widget can override one slot and still inherit the app
 * theme (including dark mode) for the rest.
 */

export const TABLE_COLOR_PARTS = [
  { key: "header_bg", label: "Latar header" },
  { key: "header_fg", label: "Teks header" },
  { key: "body_bg", label: "Latar baris" },
  { key: "stripe_bg", label: "Baris selang-seling" },
  { key: "text_fg", label: "Teks isi" },
  { key: "accent", label: "Sorotan filter" },
] as const;

export type TableColorKey = (typeof TABLE_COLOR_PARTS)[number]["key"];

const THEME_FALLBACK: Record<TableColorKey, string> = {
  header_bg: "var(--secondary)",
  header_fg: "var(--secondary-foreground)",
  body_bg: "var(--card)",
  stripe_bg: "color-mix(in oklab, var(--muted) 60%, transparent)",
  text_fg: "var(--muted-foreground)",
  accent: "var(--accent)",
};

export type TablePreset = {
  name: string;
  /** null means "inherit the app theme": applying it clears every color slot. */
  colors: Record<TableColorKey, string> | null;
};

export const TABLE_PRESETS: TablePreset[] = [
  { name: "Ikut tema", colors: null },
  {
    name: "Slate",
    colors: {
      header_bg: "#1e293b",
      header_fg: "#f1f5f9",
      body_bg: "#ffffff",
      stripe_bg: "#f8fafc",
      text_fg: "#334155",
      accent: "#dbeafe",
    },
  },
  {
    name: "Ocean",
    colors: {
      header_bg: "#0e7490",
      header_fg: "#ecfeff",
      body_bg: "#ffffff",
      stripe_bg: "#f0f9ff",
      text_fg: "#0c4a6e",
      accent: "#a5f3fc",
    },
  },
  {
    name: "Forest",
    colors: {
      header_bg: "#14532d",
      header_fg: "#f0fdf4",
      body_bg: "#ffffff",
      stripe_bg: "#f0fdf4",
      text_fg: "#1f2937",
      accent: "#bbf7d0",
    },
  },
  {
    name: "Sunset",
    colors: {
      header_bg: "#7c2d12",
      header_fg: "#fff7ed",
      body_bg: "#fffbf5",
      stripe_bg: "#fff7ed",
      text_fg: "#431407",
      accent: "#fed7aa",
    },
  },
];

function cssName(key: TableColorKey): string {
  return `--dt-${key.replace(/_/g, "-")}`;
}

export function tableCssVars(style?: T.TableStyle): CSSProperties {
  const vars: Record<string, string> = {};
  for (const { key } of TABLE_COLOR_PARTS) {
    const custom = style?.[key];
    vars[cssName(key)] = isHexColor(custom) ? custom : THEME_FALLBACK[key];
  }
  // Derived from --dt-accent, so custom highlight colors move hover with them.
  vars["--dt-hover"] = "color-mix(in oklab, var(--dt-accent) 55%, transparent)";
  return vars as CSSProperties;
}
