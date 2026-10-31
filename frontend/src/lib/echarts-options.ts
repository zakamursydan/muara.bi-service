import type { EChartsOption, LinearGradientObject } from "echarts";
import type * as T from "@/types/api";
import { safePalette, withAlpha } from "@/lib/color";

/**
 * Deterministic tabular result -> ECharts option mapping. The same rows always
 * produce the same chart, which is what lets a widget re-render without any
 * per-chart hand-written option JSON.
 */

const FALLBACK_PALETTE = ["#0891b2", "#4f46e5", "#059669", "#d97706", "#e11d48"];

/** Palette follows the CSS tokens so dark mode re-colors charts for free. */
export function resolvePalette(custom?: string[]): string[] {
  const picked = safePalette(custom);
  if (picked.length) return picked;
  if (typeof window === "undefined") return FALLBACK_PALETTE;
  const styles = getComputedStyle(document.documentElement);
  const read = (token: string) => styles.getPropertyValue(token).trim();
  const colors = [1, 2, 3, 4, 5].map((n) => read(`--chart-${n}`)).filter(Boolean);
  return colors.length ? colors : FALLBACK_PALETTE;
}

type CanvasTheme = {
  surface: string;
  text: string;
  muted: string;
  line: string;
};

/** Mirrors the light tokens in globals.css; used for SSR and unreadable props. */
const INLINE_THEME: CanvasTheme = {
  surface: "#ffffff",
  text: "#0f172a",
  muted: "#475569",
  line: "#e2e8f0",
};

/**
 * ECharts paints to a canvas, so it cannot inherit CSS variables the way DOM
 * widgets do. Reading the resolved tokens per build keeps charts correct when
 * the theme flips, without registering an ECharts theme.
 */
export function readCanvasTheme(): CanvasTheme {
  if (typeof window === "undefined") return INLINE_THEME;
  const styles = getComputedStyle(document.documentElement);
  const read = (token: string, fallback: string) =>
    styles.getPropertyValue(token).trim() || fallback;
  return {
    surface: read("--card", INLINE_THEME.surface),
    text: read("--card-foreground", INLINE_THEME.text),
    muted: read("--muted-foreground", INLINE_THEME.muted),
    line: read("--border", INLINE_THEME.line),
  };
}

function fade(color: string, from = 1, to = 0.7): LinearGradientObject {
  return {
    type: "linear",
    x: 0,
    y: 0,
    x2: 0,
    y2: 1,
    colorStops: [
      { offset: 0, color: withAlpha(color, from) },
      { offset: 1, color: withAlpha(color, to) },
    ],
  };
}

const NUMERIC_TYPES = new Set([
  "int8",
  "int16",
  "int32",
  "int",
  "int64",
  "hugeint",
  "smallint",
  "integer",
  "bigint",
  "utinyint",
  "uinteger",
  "ubigint",
  "float",
  "float4",
  "float8",
  "double",
  "real",
  "decimal",
  "numeric",
  "tinyint",
]);

export function isNumericColumn(type: string): boolean {
  const base = type.toLowerCase().split("(")[0].trim();
  return NUMERIC_TYPES.has(base) || /^decimal\(\d+,\d+\)$/.test(base);
}

export function toNumber(value: unknown): number | null {
  if (value === null || value === undefined || value === "") return null;
  const num = typeof value === "number" ? value : Number(value);
  return Number.isFinite(num) ? num : null;
}

export function formatNumber(value: unknown, decimals = 2, unit?: string): string {
  const num = toNumber(value);
  if (num === null) return "-";
  const formatted = num.toLocaleString("en-US", {
    minimumFractionDigits: Number.isInteger(num) ? 0 : decimals,
    maximumFractionDigits: decimals,
  });
  return unit ? `${formatted} ${unit}` : formatted;
}

const compactFormat = new Intl.NumberFormat("en-US", {
  notation: "compact",
  maximumFractionDigits: 1,
});

/**
 * Axis ticks have to stay inside the grid: from ten thousand up they collapse to
 * 12.3K / 4.5M rather than pushing the plot area sideways.
 */
function axisValue(value: number, decimals: number): string {
  return Math.abs(value) >= 10_000
    ? compactFormat.format(value)
    : formatNumber(value, decimals);
}

function columnsOf(result: { columns: T.ColumnMeta[] }): Map<string, string> {
  return new Map(result.columns.map((c) => [c.name, c.type]));
}

/** Which temporal display a column type needs; null means "show it verbatim". */
export function temporalKind(type: string): "date" | "time" | null {
  const upper = type.toUpperCase();
  if (upper.startsWith("TIMESTAMP") || upper.startsWith("DATETIME")) return "time";
  return upper === "DATE" ? "date" : null;
}

/**
 * Trims the driver's ISO text instead of parsing it, so a stored date is shown
 * exactly as the warehouse wrote it rather than shifted into browser timezone.
 */
export function formatTemporal(value: string, kind: "date" | "time"): string {
  const matched = /^(\d{4}-\d{2}-\d{2})[T ](\d{2}:\d{2}(?::\d{2})?)/.exec(value);
  if (!matched) return value;
  if (kind === "date") return matched[1];
  const time = matched[2];
  const hasSeconds = time.length > 5 && !time.endsWith(":00");
  return `${matched[1]} ${time.slice(0, hasSeconds ? 8 : 5)}`;
}

/** Axis labels: an all-day timestamp reads as a date, not as 00:00 noise. */
function categoryLabel(value: string, kind: "date" | "time" | null): string {
  if (!kind) return value;
  const trimmed = formatTemporal(value, kind);
  return kind === "time" && trimmed.endsWith(" 00:00") ? trimmed.slice(0, 10) : trimmed;
}

/**
 * First non-numeric column is the category axis; every numeric column becomes a
 * series. An explicit mapping in the widget's echarts_config always wins.
 */
export function inferFields(
  result: { columns: T.ColumnMeta[] },
  config?: T.EchartsConfig,
): { xField: string; yFields: string[] } {
  const types = columnsOf(result);
  const numeric = result.columns.filter((c) => isNumericColumn(c.type)).map((c) => c.name);
  const categorical = result.columns.filter((c) => !isNumericColumn(c.type)).map((c) => c.name);

  const xField = config?.x_field && types.has(config.x_field) ? config.x_field : categorical[0] ?? numeric[0];
  const yFields =
    config?.y_fields?.length
      ? config.y_fields.filter((field) => types.has(field))
      : numeric.filter((field) => field !== xField);

  return { xField, yFields };
}

export type ChartBuildInput = {
  chartType: T.ChartType;
  result: { columns: T.ColumnMeta[]; rows: Record<string, unknown>[] };
  config?: T.EchartsConfig;
  /** Category value this widget itself selected: highlight it, dim the rest. */
  highlight?: { field: string; value: string };
};

export function buildEchartsOption(input: ChartBuildInput): EChartsOption | null {
  const { chartType, result, config } = input;
  if (chartType === "table" || chartType === "kpi") return null;
  if (!result.columns.length || !result.rows.length) return null;

  const { xField, yFields } = inferFields(result, config);
  if (!xField) return null;

  const theme = readCanvasTheme();
  const palette = resolvePalette(config?.palette);
  const colorAt = (index: number) => palette[index % palette.length] ?? FALLBACK_PALETTE[0];
  const rows = result.rows;
  const categories = rows.map((row) => String(row[xField] ?? "-"));
  const decimals = config?.decimals ?? 2;
  const multiSeries = yFields.length > 1;

  const tooltip = (
    trigger: "axis" | "item",
    pointer?: Record<string, unknown>,
  ) => ({
    trigger,
    backgroundColor: theme.surface,
    borderColor: theme.line,
    borderWidth: 1,
    padding: [8, 10] as [number, number],
    textStyle: { color: theme.text, fontSize: 12 },
    extraCssText: "border-radius:10px;box-shadow:0 10px 30px -14px rgba(15,23,42,.45);",
    valueFormatter: (value: unknown) => formatNumber(value, decimals, config?.unit),
    ...(pointer ? { axisPointer: pointer } : {}),
  });

  const legend = {
    show: multiSeries,
    type: "scroll" as const,
    top: 2,
    left: "center" as const,
    icon: "roundRect" as const,
    itemWidth: 10,
    itemHeight: 10,
    itemGap: 14,
    textStyle: { fontSize: 11, color: theme.muted },
    pageIconStyle: { color: theme.muted, textStyle: { color: theme.text, fontSize: 10 } },
    pageTextStyle: { color: theme.muted, fontSize: 10 },
  };

  const base = {
    color: palette,
    animationDuration: 350,
    animationEasing: "cubicOut" as const,
    animationDurationUpdate: 250,
    // ECharts 6 replaced containLabel: keeping the axis labels inside the outer
    // bounds means a long tick narrows the plot instead of getting clipped.
    grid: {
      left: 6,
      right: 18,
      top: multiSeries ? 46 : 24,
      bottom: 6,
      outerBoundsMode: "same" as const,
      outerBoundsContain: "axisLabel" as const,
    },
    tooltip: tooltip(
      "axis",
      chartType === "bar"
        ? { type: "shadow", shadowStyle: { color: withAlpha(theme.muted, 0.08) } }
        : {
            type: "line",
            lineStyle: { color: withAlpha(theme.muted, 0.4), type: "dashed" as const },
          },
    ),
    legend,
  };

  const xKind = temporalKind(columnsOf(result).get(xField) ?? "");
  const categoryAxis = {
    type: "category" as const,
    data: categories,
    boundaryGap: chartType === "bar",
    axisLine: { lineStyle: { color: theme.line } },
    axisTick: { show: false },
    axisLabel: {
      color: theme.muted,
      fontSize: 11,
      hideOverlap: true,
      rotate: categories.length > 12 ? 30 : 0,
      formatter: (value: string) => categoryLabel(value, xKind),
    },
  };

  const valueAxis = {
    type: "value" as const,
    axisLine: { show: false },
    axisTick: { show: false },
    axisLabel: {
      color: theme.muted,
      fontSize: 11,
      formatter: (value: number) => axisValue(value, decimals),
    },
    splitLine: { lineStyle: { color: theme.line, type: "dashed" as const } },
    nameTextStyle: { color: theme.muted, fontSize: 11 },
  };

  const dimmed = (name: string) =>
    input.highlight && input.highlight.field === xField && input.highlight.value !== name;

  const seriesData = (field: string) =>
    rows.map((row) => {
      const value = toNumber(row[field]);
      const name = String(row[xField] ?? "-");
      return dimmed(name)
        ? { value, itemStyle: { opacity: 0.28 } }
        : value;
    });

  if (chartType === "pie") {
    const field = yFields[0];
    if (!field) return null;
    const data = rows.map((row) => ({
      name: String(row[xField] ?? "-"),
      value: toNumber(row[field]) ?? 0,
    }));
    const total = data.reduce((sum, item) => sum + item.value, 0);
    return {
      ...base,
      tooltip: tooltip("item"),
      // A centred donut with a bottom legend survives any card aspect ratio;
      // a side legend pushed the ring outside the canvas on wide widgets.
      legend: { ...legend, show: true, top: undefined, bottom: 2 },
      title: {
        text: formatNumber(total, decimals, config?.unit),
        subtext: "Total",
        left: "50%",
        top: "40%",
        textAlign: "center",
        textStyle: { fontSize: 18, fontWeight: 600, color: theme.text },
        subtextStyle: { fontSize: 11, color: theme.muted },
      },
      series: [
        {
          type: "pie",
          center: ["50%", "46%"],
          radius: ["50%", "74%"],
          padAngle: 2,
          avoidLabelOverlap: true,
          itemStyle: { borderColor: theme.surface, borderWidth: 2, borderRadius: 6 },
          label: { show: false },
          labelLine: { show: false },
          emphasis: {
            scaleSize: 6,
            label: {
              show: true,
              position: "center",
              formatter: config?.show_percent === false ? "{b}\n{c}" : "{b}\n{d}%",
              fontSize: 12,
              fontWeight: 600,
              color: theme.text,
            },
          },
          data,
        },
      ],
    };
  }

  if (chartType === "scatter") {
    const field = yFields[0];
    if (!field) return null;
    return {
      ...base,
      tooltip: tooltip("item"),
      xAxis: { ...valueAxis, name: xField, nameLocation: "middle" as const, nameGap: 26 },
      yAxis: { ...valueAxis, name: field },
      series: [
        {
          type: "scatter",
          symbolSize: 10,
          itemStyle: {
            color: withAlpha(colorAt(0), 0.85),
            borderColor: theme.surface,
            borderWidth: 1,
          },
          emphasis: {
            itemStyle: {
              color: withAlpha(colorAt(0), 1),
              borderColor: theme.text,
              borderWidth: 1,
            },
          },
          data: rows.map((row) => [toNumber(row[xField]), toNumber(row[field])]),
        },
      ],
    };
  }

  const lineLike = chartType === "line" || chartType === "area";
  const stacked = Boolean(config?.stacked) && chartType === "bar";
  return {
    ...base,
    xAxis: categoryAxis,
    yAxis: valueAxis,
    series: yFields.map((field, index) => {
      const color = colorAt(index);
      const isTop = index === yFields.length - 1;
      return {
        name: field,
        type: lineLike ? ("line" as const) : ("bar" as const),
        stack: stacked ? "total" : undefined,
        smooth: lineLike ? Boolean(config?.smooth) : undefined,
        itemStyle: lineLike
          ? { color }
          : {
              color: fade(color),
              borderRadius: stacked && !isTop ? 0 : [5, 5, 0, 0],
            },
        barMaxWidth: lineLike ? undefined : 34,
        lineStyle: lineLike ? { width: 2.5, cap: "round" as const } : undefined,
        symbol: "circle",
        symbolSize: 6,
        showSymbol: lineLike ? rows.length <= 40 : undefined,
        areaStyle:
          chartType === "area" ? { color: fade(color, 0.32, 0.02), origin: "start" as const } : undefined,
        emphasis: {
          focus: "series" as const,
          blur: { itemStyle: { opacity: 0.35 }, lineStyle: { opacity: 0.35 } },
        },
        data: seriesData(field),
      };
    }),
  };
}

/** Single big number for a KPI card: first measure of the first row. */
export function pickKpi(
  result: { columns: T.ColumnMeta[]; rows: Record<string, unknown>[] },
  config?: T.EchartsConfig,
): { label: string; value: string } {
  const row = result.rows[0];
  if (!row) return { label: "-", value: "-" };
  const field = config?.y_fields?.[0] ?? result.columns.find((c) => isNumericColumn(c.type))?.name;
  if (!field) return { label: result.columns[0]?.name ?? "-", value: "-" };
  return { label: field, value: formatNumber(row[field], config?.decimals ?? 2, config?.unit) };
}
