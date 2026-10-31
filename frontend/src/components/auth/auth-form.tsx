"use client";

import { type ReactNode } from "react";
import Link from "next/link";
import { Database } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

type AuthFormProps = {
  title: string;
  subtitle: string;
  submitLabel: string;
  pending: boolean;
  error: string | null;
  onSubmit: () => void;
  children: ReactNode;
  footer: ReactNode;
};

export function AuthForm({
  title,
  subtitle,
  submitLabel,
  pending,
  error,
  onSubmit,
  children,
  footer,
}: AuthFormProps) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <Card className="w-full max-w-md gap-0 overflow-hidden p-0 shadow-sm">
        <div className="flex items-center gap-2 border-b border-border bg-secondary/50 px-6 py-4">
          <div className="grid size-8 place-items-center rounded-lg bg-primary text-primary-foreground">
            <Database className="size-4" />
          </div>
          <div>
            <p className="text-sm font-semibold">muara.bi</p>
            <p className="text-[11px] text-muted-foreground">Unified BI &amp; AI Analytics on DuckDB</p>
          </div>
        </div>

        <form
          className="flex flex-col gap-4 p-6"
          onSubmit={(event) => {
            event.preventDefault();
            onSubmit();
          }}
        >
          <div className="flex flex-col gap-1">
            <h1 className="text-lg font-semibold text-foreground">{title}</h1>
            <p className="text-sm text-muted-foreground">{subtitle}</p>
          </div>

          {children}

          {error && (
            <p className="rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">{error}</p>
          )}

          <Button type="submit" className="w-full" disabled={pending}>
            {pending ? "Memproses…" : submitLabel}
          </Button>

          <div className="text-center text-xs text-muted-foreground">{footer}</div>
        </form>
      </Card>
    </div>
  );
}

export function Field({
  id,
  label,
  type = "text",
  value,
  onChange,
  autoComplete,
  placeholder,
}: {
  id: string;
  label: string;
  type?: string;
  value: string;
  onChange: (value: string) => void;
  autoComplete?: string;
  placeholder?: string;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <Input
        id={id}
        type={type}
        value={value}
        autoComplete={autoComplete}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        required
      />
    </div>
  );
}

export function AuthFooter({ mode }: { mode: "login" | "register" }) {
  return mode === "login" ? (
    <>
      Belum punya akun?{" "}
      <Link className="font-medium text-primary underline-offset-2 hover:underline" href="/register">
        Daftar workspace
      </Link>
    </>
  ) : (
    <>
      Sudah punya akun?{" "}
      <Link className="font-medium text-primary underline-offset-2 hover:underline" href="/login">
        Masuk
      </Link>
    </>
  );
}
