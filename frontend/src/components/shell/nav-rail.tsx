"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Boxes,
  Database,
  LayoutGrid,
  PanelLeftClose,
  PanelLeftOpen,
  Plug,
  Settings,
  Sparkles,
} from "lucide-react";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

const items = [
  { href: "/catalog", label: "Catalog", icon: LayoutGrid },
  { href: "/semantic-models", label: "Semantic Models", icon: Boxes },
  { href: "/studio", label: "Data Studio", icon: Sparkles },
  { href: "/sources", label: "Data Sources", icon: Plug },
  { href: "/settings", label: "Settings", icon: Settings },
];

const RAIL_WIDTH_KEY = "muara.bi.rail";

export function NavRail() {
  const pathname = usePathname();
  // Only mounts after the shell's hydration guard, so reading storage during
  // render cannot cause a hydration mismatch.
  const [expanded, setExpanded] = useState(() => {
    const stored = window.localStorage.getItem(RAIL_WIDTH_KEY);
    return stored === null ? true : stored === "1";
  });

  const toggle = () => {
    const next = !expanded;
    window.localStorage.setItem(RAIL_WIDTH_KEY, next ? "1" : "0");
    setExpanded(next);
  };

  return (
    <nav
      className={cn(
        "flex h-full shrink-0 flex-col border-r border-sidebar-border bg-sidebar transition-[width] duration-200",
        expanded ? "w-[220px]" : "w-16",
      )}
    >
      <div
        className={cn(
          "flex items-center gap-2 px-3 py-4",
          !expanded && "justify-center",
        )}
      >
        <div className="grid size-8 shrink-0 place-items-center rounded-lg bg-primary text-primary-foreground">
          <Database className="size-4" />
        </div>
        {expanded && (
          <div className="min-w-0">
            <p className="truncate text-sm font-semibold text-foreground">
              muara.bi
            </p>
            <p className="truncate text-[11px] text-muted-foreground">
              BI &amp; AI Analytics
            </p>
          </div>
        )}
      </div>

      <ul className="flex flex-1 flex-col gap-1 px-2">
        {items.map((item) => {
          const active =
            pathname === item.href || pathname.startsWith(`${item.href}/`);
          const link = (
            <Link
              href={item.href}
              className={cn(
                "flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm font-medium transition-colors",
                !expanded && "justify-center px-0",
                active
                  ? "bg-primary/10 text-primary"
                  : "text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
              )}
            >
              <item.icon className="size-4 shrink-0" />
              {expanded && <span className="truncate">{item.label}</span>}
            </Link>
          );
          return (
            <li key={item.href}>
              {expanded ? (
                link
              ) : (
                <Tooltip>
                  <TooltipTrigger asChild>{link}</TooltipTrigger>
                  <TooltipContent side="right">{item.label}</TooltipContent>
                </Tooltip>
              )}
            </li>
          );
        })}
      </ul>

      <button
        type="button"
        className="flex items-center gap-2 border-t border-sidebar-border px-4 py-3 text-xs text-muted-foreground hover:bg-sidebar-accent"
        onClick={toggle}
        aria-label={expanded ? "Persempit navigasi" : "Lebarkan navigasi"}
      >
        {expanded ? (
          <PanelLeftClose className="size-4" />
        ) : (
          <PanelLeftOpen className="size-4" />
        )}
        {expanded && <span>Tutup panel</span>}
      </button>
    </nav>
  );
}
