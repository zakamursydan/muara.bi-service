"use client";

import { useEffect, useRef } from "react";
import * as echarts from "echarts/core";
import { BarChart, LineChart, PieChart, ScatterChart } from "echarts/charts";
import {
  GridComponent,
  LegendComponent,
  MarkLineComponent,
  TitleComponent,
  TooltipComponent,
} from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import type { EChartsOption } from "echarts";
import { cn } from "@/lib/utils";

echarts.use([
  BarChart,
  LineChart,
  PieChart,
  ScatterChart,
  GridComponent,
  LegendComponent,
  TitleComponent,
  TooltipComponent,
  MarkLineComponent,
  CanvasRenderer,
]);

type ChartRendererProps = {
  option: EChartsOption;
  /** Column the click value belongs to, used to name the cross-filter parameter. */
  categoryField?: string;
  onCategoryClick?: (field: string, value: string) => void;
  className?: string;
};

/**
 * ECharts bound to a ResizeObserver instead of a window listener, so dragging a
 * grid card resizes at frame rate without re-mounting the chart.
 */
export function ChartRenderer({
  option,
  categoryField,
  onCategoryClick,
  className,
}: ChartRendererProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<echarts.ECharts | null>(null);
  const handlersRef = useRef({ onCategoryClick, categoryField });

  useEffect(() => {
    handlersRef.current = { onCategoryClick, categoryField };
  }, [onCategoryClick, categoryField]);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    const chart = echarts.init(el, undefined, { renderer: "canvas" });
    chartRef.current = chart;
    chart.on("click", (params) => {
      const { onCategoryClick: click, categoryField: field } = handlersRef.current;
      const value = typeof params.name === "string" ? params.name : undefined;
      if (field && value) click?.(field, value);
    });

    const observer = new ResizeObserver(() => chart.resize());
    observer.observe(el);

    return () => {
      observer.disconnect();
      chart.dispose();
      chartRef.current = null;
    };
  }, []);

  useEffect(() => {
    chartRef.current?.setOption(option, { notMerge: true });
  }, [option]);

  return <div ref={containerRef} className={cn("h-full w-full", className)} />;
}
