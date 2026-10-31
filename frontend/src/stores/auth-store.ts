import { create } from "zustand";
import { api, getToken, setToken, ApiError } from "@/lib/api";
import type * as T from "@/types/api";

type AuthStatus = "unknown" | "authenticated" | "anonymous";

type AuthState = {
  status: AuthStatus;
  user: T.User | null;
  workspaces: T.Workspace[];
  error: string | null;
  pending: boolean;
  /** Replays the stored JWT against /auth/me once per app load. */
  bootstrap: () => Promise<void>;
  login: (body: T.LoginRequest) => Promise<boolean>;
  register: (body: T.RegisterRequest) => Promise<boolean>;
  logout: () => void;
};

function messageOf(err: unknown): string {
  return err instanceof ApiError ? err.message : "Request failed";
}

export const useAuthStore = create<AuthState>((set, get) => ({
  status: "unknown",
  user: null,
  workspaces: [],
  error: null,
  pending: false,

  bootstrap: async () => {
    if (get().status !== "unknown") return;
    if (!getToken()) {
      set({ status: "anonymous" });
      return;
    }
    try {
      const profile = await api.auth.me();
      set({ status: "authenticated", user: profile.user, workspaces: profile.workspaces });
    } catch (err) {
      // A stale or revoked JWT must not leave the app half-authenticated.
      setToken(null);
      set({ status: "anonymous", user: null, workspaces: [], error: messageOf(err) });
    }
  },

  login: async (body) => {
    set({ pending: true, error: null });
    try {
      const res = await api.auth.login(body);
      setToken(res.token);
      set({
        status: "authenticated",
        user: res.user,
        workspaces: res.workspaces,
        pending: false,
      });
      return true;
    } catch (err) {
      set({ pending: false, error: messageOf(err), status: "anonymous" });
      return false;
    }
  },

  register: async (body) => {
    set({ pending: true, error: null });
    try {
      const res = await api.auth.register(body);
      setToken(res.token);
      set({
        status: "authenticated",
        user: res.user,
        workspaces: res.workspaces,
        pending: false,
      });
      return true;
    } catch (err) {
      set({ pending: false, error: messageOf(err) });
      return false;
    }
  },

  logout: () => {
    setToken(null);
    set({ status: "anonymous", user: null, workspaces: [], error: null });
  },
}));
