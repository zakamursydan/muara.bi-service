import { create } from "zustand";

/**
 * Bumped by the header "Sync / Force Refresh" action. Widget query keys include
 * the nonce, so one bump re-issues every visible query with X-Force-Refresh and
 * bypasses the Redis result cache.
 */
type RefreshState = {
  nonce: number;
  bump: () => void;
};

export const useRefreshStore = create<RefreshState>((set) => ({
  nonce: 0,
  bump: () => set((state) => ({ nonce: state.nonce + 1 })),
}));
