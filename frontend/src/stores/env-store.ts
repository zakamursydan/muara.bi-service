import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";

type EnvState = {
  /** Active custom environment tag (production, staging, uat, anything). */
  envTag: string | null;
  setEnvTag: (envTag: string) => void;
  clear: () => void;
};

/**
 * The tag is chosen globally and resolved per connection at query time, so a
 * dashboard never has to name an environment in its SQL.
 */
export const useEnvStore = create<EnvState>()(
  persist(
    (set) => ({
      envTag: null,
      setEnvTag: (envTag) => set({ envTag }),
      clear: () => set({ envTag: null }),
    }),
    {
      name: "muara.bi.env",
      storage: createJSONStorage(() => localStorage),
    },
  ),
);
