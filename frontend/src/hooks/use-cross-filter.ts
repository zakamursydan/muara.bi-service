"use client";

import { useCallback } from "react";
import { crossFilterParam, useFilterStore } from "@/stores/filter-store";

/**
 * Cross-filter bindings for one widget. The widget that emitted the selection is
 * `isSource` and stays unfiltered, so clicking a bar never collapses its own
 * chart down to that single bar.
 */
export function useCrossFilter(widgetId: string) {
  const cross = useFilterStore((state) => state.cross);
  const toggleCrossFilter = useFilterStore((state) => state.toggleCrossFilter);
  const clearCrossFilter = useFilterStore((state) => state.clearCrossFilter);

  const isSource = cross?.sourceWidgetId === widgetId;
  const isFilteredByOther = Boolean(cross) && !isSource;

  const toggle = useCallback(
    (column: string, value: unknown, label = String(value)) =>
      toggleCrossFilter({
        sourceWidgetId: widgetId,
        column,
        param: crossFilterParam(column),
        value,
        label,
      }),
    [toggleCrossFilter, widgetId],
  );

  const isActive = useCallback(
    (column: string, value: unknown) =>
      isSource && cross?.param === crossFilterParam(column) && cross?.value === value,
    [cross, isSource],
  );

  return {
    isSource,
    isFilteredByOther,
    activeLabel: isSource ? cross?.label : undefined,
    activeColumn: isSource && cross ? cross.column : undefined,
    activeValue: isSource && cross ? String(cross.value) : undefined,
    toggle,
    isActive,
    clear: clearCrossFilter,
  };
}
