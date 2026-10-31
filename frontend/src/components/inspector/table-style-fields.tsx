"use client";

import { Palette } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  TABLE_COLOR_PARTS,
  TABLE_PRESETS,
  type TableColorKey,
} from "@/lib/table-theme";
import { isHexColor } from "@/lib/color";
import { cn } from "@/lib/utils";
import type * as T from "@/types/api";

type TableStyleFieldsProps = {
  value?: T.TableStyle;
  onChange: (next: T.TableStyle) => void;
};

const NEUTRAL = "#e2e8f0";

export function TableStyleFields({ value, onChange }: TableStyleFieldsProps) {
  const style = value ?? {};

  const patch = (part: Partial<T.TableStyle>) => onChange({ ...style, ...part });

  const setColor = (key: TableColorKey, hex: string | undefined) =>
    patch({ [key]: hex } as Partial<T.TableStyle>);

  const applyPreset = (colors: Record<TableColorKey, string> | null) => {
    const cleared = Object.fromEntries(
      TABLE_COLOR_PARTS.map((part) => [part.key, undefined]),
    ) as Partial<T.TableStyle>;
    patch({ ...cleared, ...(colors ?? {}) });
  };

  return (
    <div className="col-span-2 flex flex-col gap-2.5 rounded-md border border-border bg-muted/30 p-2.5">
      <div className="flex items-center gap-1.5 text-xs font-medium text-foreground">
        <Palette className="size-3.5 text-primary" />
        Tampilan tabel
      </div>

      <div className="flex flex-wrap gap-1">
        {TABLE_PRESETS.map((preset) => (
          <Button
            key={preset.name}
            variant="outline"
            size="xs"
            title={preset.colors ? `Preset ${preset.name}` : "Kembali ke warna tema"}
            onClick={() => applyPreset(preset.colors)}
          >
            {preset.colors && (
              <span className="flex overflow-hidden rounded-sm">
                {[preset.colors.header_bg, preset.colors.stripe_bg, preset.colors.accent].map(
                  (color) => (
                    <span
                      key={color}
                      className="size-2.5"
                      style={{ backgroundColor: color }}
                    />
                  ),
                )}
              </span>
            )}
            {preset.name}
          </Button>
        ))}
      </div>

      <div className="grid grid-cols-3 gap-2">
        <label className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
          <Switch
            size="sm"
            checked={style.density === "cozy"}
            onCheckedChange={(checked) => patch({ density: checked ? "cozy" : "compact" })}
          />
          Renggang
        </label>
        <label className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
          <Switch
            size="sm"
            checked={Boolean(style.zebra)}
            onCheckedChange={(checked) => patch({ zebra: checked })}
          />
          Selang-seling
        </label>
        <label className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
          <Switch
            size="sm"
            checked={style.sticky_header !== false}
            onCheckedChange={(checked) => patch({ sticky_header: checked })}
          />
          Header melekat
        </label>
      </div>

      <label className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
        <Switch
          size="sm"
          checked={Boolean(style.row_numbers)}
          onCheckedChange={(checked) => patch({ row_numbers: checked })}
        />
        Nomor baris
      </label>

      <div className="grid grid-cols-2 gap-1.5">
        {TABLE_COLOR_PARTS.map((part) => {
          const current = style[part.key];
          const invalid = current !== undefined && !isHexColor(current);
          return (
            <div key={part.key} className="flex items-center gap-1.5">
              <input
                type="color"
                aria-label={part.label}
                value={isHexColor(current) ? current : NEUTRAL}
                onChange={(event) => setColor(part.key, event.target.value)}
                className="size-6 shrink-0 cursor-pointer rounded border border-border bg-transparent p-0"
              />
              <div className="flex min-w-0 flex-col gap-0.5">
                <span className="truncate text-[10px] leading-none text-muted-foreground">
                  {part.label}
                </span>
                <Input
                  value={current ?? ""}
                  placeholder="ikut tema"
                  onChange={(event) => setColor(part.key, event.target.value || undefined)}
                  className={cn(
                    "h-6 px-1.5 font-mono text-[10px]",
                    invalid && "border-destructive",
                  )}
                />
              </div>
            </div>
          );
        })}
      </div>

      <p className="text-[10px] leading-tight text-muted-foreground">
        Warna kosong mengikuti tema aplikasi, jadi dark mode tetap terbaca.
      </p>
    </div>
  );
}
