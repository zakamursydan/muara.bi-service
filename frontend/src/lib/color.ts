/**
 * Widget colors are stored per dashboard and replayed on anonymous share pages,
 * so a config value only ever becomes a CSS/canvas color when it is a literal
 * hex. Anything else falls back to the app theme.
 */
const HEX = /^#(?:[0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$/i;

export function isHexColor(value?: string): value is string {
  return Boolean(value && HEX.test(value));
}

export function safePalette(colors?: string[]): string[] {
  return (colors ?? []).filter(isHexColor);
}

/** Blend a hex color with transparency. Non-hex input is returned unchanged. */
export function withAlpha(color: string, alpha: number): string {
  if (!isHexColor(color)) return color;
  let body = color.slice(1);
  body =
    body.length === 3 || body.length === 4
      ? body
          .slice(0, 3)
          .split("")
          .map((char) => char + char)
          .join("")
      : body.slice(0, 6);
  const [r, g, b] = [0, 2, 4].map((start) => parseInt(body.slice(start, start + 2), 16));
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}
