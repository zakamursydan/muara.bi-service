import type * as Monaco from "monaco-editor";
import type { Monaco as MonacoInstance } from "@monaco-editor/react";
import type * as T from "@/types/api";

type CompletionRegistry = {
  tables: () => { qualified: string; table: string; columns: T.ColumnMeta[] }[];
};

let registered = false;

/**
 * Registers a DuckDB-flavoured SQL completion provider once per Monaco mount.
 * The table list is read through a getter because the schema store changes
 * while the editor instance stays alive.
 */
export function registerSqlCompletion(monaco: MonacoInstance, registry: CompletionRegistry) {
  if (registered) return;
  registered = true;

  monaco.languages.registerCompletionItemProvider("sql", {
    triggerCharacters: [" ", ".", "_"],
    provideCompletionItems: (
      model: Monaco.editor.ITextModel,
      position: Monaco.Position,
    ) => {
      const word = model.getWordUntilPosition(position);
      const range = {
        startLineNumber: position.lineNumber,
        endLineNumber: position.lineNumber,
        startColumn: word.startColumn,
        endColumn: word.endColumn,
      };

      const line = model.getValueInRange({
        startLineNumber: position.lineNumber,
        startColumn: 1,
        endLineNumber: position.lineNumber,
        endColumn: position.column,
      });
      const tables = registry.tables();
      const dotted = /([A-Za-z0-9_.]+)\.\s*$/.exec(line);
      const suggestions: Monaco.languages.CompletionItem[] = [];

      if (dotted) {
        const prefix = dotted[1].toLowerCase();
        const owner = tables.find(
          (table) => table.qualified.toLowerCase() === prefix || table.table.toLowerCase() === prefix,
        );
        if (owner) {
          for (const column of owner.columns) {
            suggestions.push({
              label: column.name,
              kind: monaco.languages.CompletionItemKind.Field,
              insertText: column.name,
              detail: column.type,
              range,
            });
          }
        }
      }

      for (const table of tables) {
        suggestions.push({
          label: table.qualified,
          kind: monaco.languages.CompletionItemKind.Struct,
          insertText: table.qualified,
          detail: `${table.columns.length} kolom`,
          range,
        });
      }

      for (const keyword of [
        "SELECT",
        "FROM",
        "WHERE",
        "GROUP BY",
        "ORDER BY",
        "HAVING",
        "JOIN",
        "LEFT JOIN",
        "ON",
        "AS",
        "LIMIT",
        "WITH",
        "SUM(",
        "COUNT(",
        "AVG(",
        "DATE_TRUNC(",
      ]) {
        suggestions.push({
          label: keyword,
          kind: monaco.languages.CompletionItemKind.Keyword,
          insertText: keyword,
          range,
        });
      }

      return { suggestions };
    },
  });
}
