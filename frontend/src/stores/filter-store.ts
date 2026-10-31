import { create } from "zustand";

export type CrossSelection = {
  /** Widget the click came from; it must not be filtered by its own click. */
  sourceWidgetId: string;
  /** Column the click came from, e.g. `orders.brand`. */
  column: string;
  /** Builder parameter name, i.e. `flt_<column>` with dots flattened. */
  param: string;
  value: unknown;
  /** Display label of the selected category, used for the chip in the filter bar. */
  label: string;
};

type FilterState = {
  /** Values for every `{{param}}` placeholder, global filters and cross-filters alike. */
  values: Record<string, unknown>;
  cross: CrossSelection | null;
  setValue: (param: string, value: unknown) => void;
  clearValue: (param: string) => void;
  /** Fill only params the user has not touched, so re-renders keep edits. */
  seedValues: (defaults: Record<string, unknown>) => void;
  /** Clicking the same category again lifts the selection. */
  toggleCrossFilter: (selection: CrossSelection) => void;
  clearCrossFilter: () => void;
  reset: () => void;
};

/**
 * Pure, so a component can select `values` and `cross` (both stable references)
 * and memoise the result instead of returning a fresh object from a selector.
 *
 * The widget that produced the cross-filter ignores that selection, which is
 * what keeps its own chart from collapsing into a single bar.
 */
export function widgetParameters(
  widgetId: string,
  defaults: Record<string, unknown>,
  values: Record<string, unknown>,
  cross: CrossSelection | null,
): Record<string, unknown> {
  const next = { ...defaults, ...values };
  if (cross && cross.sourceWidgetId === widgetId) {
    if (defaults[cross.param] === undefined) delete next[cross.param];
    else next[cross.param] = defaults[cross.param];
  }
  return next;
}

/** Mirrors the backend compiler: `flt_` plus the column with dots flattened. */
export const crossFilterParam = (column: string) => `flt_${column.replace(/\./g, "_")}`;

export const useFilterStore = create<FilterState>((set, get) => ({
  values: {},
  cross: null,

  setValue: (param, value) =>
    set((state) => ({ values: { ...state.values, [param]: value } })),

  clearValue: (param) =>
    set((state) => {
      const next = { ...state.values };
      delete next[param];
      return { values: next };
    }),

  seedValues: (defaults) =>
    set((state) => {
      let changed = false;
      const next = { ...state.values };
      for (const [param, value] of Object.entries(defaults)) {
        if (next[param] === undefined) {
          next[param] = value;
          changed = true;
        }
      }
      return changed ? { values: next } : state;
    }),

  toggleCrossFilter: (selection) => {
    const current = get().cross;
    const isSame =
      current &&
      current.sourceWidgetId === selection.sourceWidgetId &&
      current.param === selection.param &&
      current.value === selection.value;

    if (isSame) {
      get().clearCrossFilter();
      return;
    }

    set((state) => ({
      values: { ...state.values, [selection.param]: selection.value },
      cross: selection,
    }));
  },

  clearCrossFilter: () =>
    set((state) => {
      const cross = state.cross;
      if (!cross) return state;
      const values = { ...state.values };
      delete values[cross.param];
      return { values, cross: null };
    }),

  reset: () => set({ values: {}, cross: null }),
}));
