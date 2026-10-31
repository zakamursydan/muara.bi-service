"use client";

import { useSyncExternalStore } from "react";

const subscribe = () => () => {};

/** False during SSR so guarded routes can render a skeleton instead of flashing a redirect. */
export function useHydrated(): boolean {
  return useSyncExternalStore(
    subscribe,
    () => true,
    () => false,
  );
}
