import type { ReactNode } from "react";

/**
 * Public surface: no auth guard and no app chrome, because share links and
 * embeds are opened by anonymous viewers.
 */
export default function PublicLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col bg-muted/30">
      <main className="mx-auto w-full max-w-[1400px] flex-1 px-4 py-6">{children}</main>
      <footer className="px-4 py-3 text-center text-[11px] text-muted-foreground">
        Dibagikan dengan muara.bi
      </footer>
    </div>
  );
}
