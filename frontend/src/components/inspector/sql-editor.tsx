"use client";

import { useEffect, useMemo, useRef } from "react";
import dynamic from "next/dynamic";
import type { OnMount } from "@monaco-editor/react";
import { registerSqlCompletion } from "@/lib/monaco-completion";
import { useSchemaTree } from "@/hooks/use-schema-tree";

const MonacoEditor = dynamic(async () => (await import("@monaco-editor/react")).default, {
  ssr: false,
  loading: () => <div className="h-60 animate-pulse rounded-md bg-secondary" />,
});

type SqlEditorProps = {
  value: string;
  onChange: (value: string) => void;
  workspaceId: string;
  projectId: string;
  envTag?: string;
  readOnly?: boolean;
};

export function SqlEditor({ value, onChange, workspaceId, projectId, envTag, readOnly }: SqlEditorProps) {
  const { tables } = useSchemaTree(workspaceId, envTag, projectId);
  const registry = useRef(tables);

  // Monaco reads the live schema through this getter, so the completion provider
  // stays registered once while the table list keeps updating.
  useEffect(() => {
    registry.current = tables;
  }, [tables]);

  // OnMount hands over (editor, monaco); only the second argument is the API namespace.
  const handleMount = useMemo<OnMount>(
    () => (_editor, monaco) => registerSqlCompletion(monaco, { tables: () => registry.current }),
    [],
  );

  return (
    <MonacoEditor
      height="240px"
      language="sql"
      value={value}
      onChange={(next: string | undefined) => onChange(next ?? "")}
      onMount={handleMount}
      options={{
        readOnly,
        minimap: { enabled: false },
        fontSize: 12,
        scrollBeyondLastLine: false,
        automaticLayout: true,
        tabSize: 2,
        wordWrap: "on",
        renderLineHighlight: "none",
      }}
    />
  );
}
