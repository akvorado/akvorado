// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import { syntaxTree } from "@codemirror/language";
import type { EditorView } from "@codemirror/view";

export const linterSource = async (view: EditorView) => {
  const code = view.state.doc.toString();
  const response = await fetch("/api/v0/console/filter/validate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ filter: code }),
  });
  if (!response.ok) return [];
  const data = await response.json();
  const diagnostic =
    data.errors?.map(
      ({ offset, message }: { offset: number; message: string }) => {
        const syntaxNode = syntaxTree(view.state).resolve(offset, 1);
        const word = view.state.wordAt(offset);
        const { from, to } = {
          from:
            (syntaxNode.name !== "Filter" && syntaxNode?.from) ||
            word?.from ||
            offset,
          to:
            (syntaxNode.name !== "Filter" && syntaxNode?.to) ||
            word?.to ||
            offset,
        };
        return {
          from: from === to ? from - 1 : from,
          to,
          severity: "error",
          message: message,
        };
      },
    ) || [];
  return diagnostic;
};
