"use client";

import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { isHexColor } from "@/lib/color";
import { resolvePalette } from "@/lib/echarts-options";
import { cn } from "@/lib/utils";

const NEUTRAL = "#94a3b8";

const PALETTE_PRESETS: { name: string; colors: string[] }[] = [
  { name: "Ombak", colors: ["#0891b2", "#4f46e5", "#059669", "#d97706", "#e11d48"] },
  { name: "Pastel", colors: ["#7dd3fc", "#c4b5fd", "#86efac", "#fcd34d", "#fda4af"] },
  { name: "Tegas", colors: ["#0f766e", "#1d4ed8", "#7c3aed", "#b45309", "#be123c"] },
  { name: "Biru", colors: ["#0c4a6e", "#0369a1", "#0284c7", "#38bdf8", "#7dd3fc"] },
];

type ChartPaletteFieldsProps = {
  value?: string[];
  onChange: (next: string[] | undefined) => void;
  disabled?: boolean;
};

/**
 * A widget palette is stored as an explicit 5-color list, so editing one slot
 * keeps the rest of the theme instead of leaving gaps in the cycle.
 */
export function ChartPaletteFields({
  value,
  onChange,
  disabled,
}: ChartPaletteFieldsProps) {
  const effective = resolvePalette(value);
  const custom = Boolean(value?.length);

  const setSlot = (index: number, hex: string) => {
    const base = resolvePalette(value);
    const next = Array.from({ length: 5 }, (_, slot) => base[slot % base.length] ?? hex);
    next[index] = hex;
    onChange(next);
  };

  return (
    <div className="col-span-2 flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <Label className="text-xs">Palet chart</Label>
        {custom && (
          <Button
            size="sm"
            variant="ghost"
            className="h-6 px-2 text-[11px]"
            disabled={disabled}
            onClick={() => onChange(undefined)}
          >
            Ikut tema
          </Button>
        )}
      </div>

      <div className="flex flex-wrap gap-1.5">
        {PALETTE_PRESETS.map((preset) => (
          <Button
            key={preset.name}
            size="sm"
            variant="outline"
            className="h-7 gap-1.5 px-2 text-[11px]"
            disabled={disabled}
            onClick={() => onChange(preset.colors)}
          >
            <span className="flex gap-0.5">
              {preset.colors.slice(0, 3).map((color) => (
                <span
                  key={color}
                  className="size-2 rounded-full"
                  style={{ background: color }}
                />
              ))}
            </span>
            {preset.name}
          </Button>
        ))}
      </div>

      <div className="grid grid-cols-5 gap-1.5">
        {effective.map((color, index) => (
          <div key={index} className="flex flex-col items-center gap-1">
            <input
              type="color"
              aria-label={`Warna series ${index + 1}`}
              className={cn(
                "h-7 w-full cursor-pointer rounded-md border border-border bg-transparent p-0.5",
                disabled && "cursor-not-allowed opacity-60",
              )}
              disabled={disabled}
              value={isHexColor(color) ? color : NEUTRAL}
              onChange={(event) => setSlot(index, event.target.value)}
            />
            <span className="font-mono text-[10px] text-muted-foreground">
              {color.toUpperCase()}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
