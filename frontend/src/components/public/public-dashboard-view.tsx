"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Loader2, Lock, ShieldQuestion } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PublicCanvas } from "@/components/public/public-canvas";
import { api, ApiError } from "@/lib/api";

type PublicDashboardViewProps = {
  /** Share token for a public link, or the signed JWT for an embed. */
  credential: string;
  mode: "share" | "embed";
};

export function PublicDashboardView({ credential, mode }: PublicDashboardViewProps) {
  // The password is only ever sent as a header; it is never persisted.
  const [password, setPassword] = useState<string | undefined>();

  const query = useQuery({
    queryKey: ["public-dashboard", mode, credential, password ?? ""],
    queryFn: () =>
      mode === "share"
        ? api.public.dashboard(credential, password)
        : api.public.embed(credential, password),
    retry: 0,
  });

  if (!credential) {
    return (
      <Card className="mx-auto flex max-w-md flex-col items-center gap-3 border-dashed px-6 py-12 text-center">
        <ShieldQuestion className="size-6 text-muted-foreground" />
        <p className="text-sm font-medium text-foreground">Token tidak ditemukan.</p>
        <p className="text-xs text-muted-foreground">
          Tautan embed harus membawa parameter token yang bertanda tangan dari pemilik dashboard.
        </p>
      </Card>
    );
  }

  if (query.isLoading) {
    return (
      <div className="flex min-h-64 items-center justify-center">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (query.isError) {
    const status = query.error instanceof ApiError ? query.error.status : 0;
    // A rejected password must return to the prompt, not to a dead-end error card.
    if (status === 401 && password) {
      return (
        <PasswordGate
          error="Kata sandi salah."
          onSubmit={setPassword}
          busy={query.isFetching}
        />
      );
    }
    return (
      <Card className="mx-auto flex max-w-md flex-col items-center gap-3 border-dashed px-6 py-12 text-center">
        <ShieldQuestion className="size-6 text-muted-foreground" />
        <p className="text-sm font-medium text-foreground">
          {status === 410 || status === 404 ? "Tautan ini tidak lagi aktif." : "Dashboard tidak dapat dimuat."}
        </p>
        <p className="text-xs text-muted-foreground">{query.error?.message}</p>
      </Card>
    );
  }

  const dashboard = query.data;
  if (!dashboard) return null;

  if (dashboard.password_required && !dashboard.widgets) {
    return (
      <PasswordGate
        onSubmit={setPassword}
        busy={query.isFetching}
      />
    );
  }

  return (
    <PublicCanvas dashboard={dashboard} password={password} compact={mode === "embed"} />
  );
}

function PasswordGate({
  onSubmit,
  error,
  busy,
}: {
  onSubmit: (password: string) => void;
  error?: string;
  busy: boolean;
}) {
  const [value, setValue] = useState("");

  return (
    <Card className="mx-auto flex max-w-sm flex-col gap-3 px-6 py-8">
      <div className="flex items-center gap-2">
        <Lock className="size-4 text-primary" />
        <p className="text-sm font-medium text-foreground">Dashboard ini dilindungi kata sandi</p>
      </div>
      <form
        className="flex flex-col gap-3"
        onSubmit={(event) => {
          event.preventDefault();
          onSubmit(value);
        }}
      >
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="share-password">Kata sandi</Label>
          <Input
            id="share-password"
            type="password"
            autoFocus
            value={value}
            onChange={(event) => setValue(event.target.value)}
          />
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>
        <Button type="submit" size="sm" disabled={!value.trim() || busy}>
          Buka dashboard
        </Button>
      </form>
    </Card>
  );
}
