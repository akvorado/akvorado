// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import { syntaxTree } from "@codemirror/language";

import type {
  CompletionContext,
  CompletionResult,
} from "@codemirror/autocomplete";
import type { SyntaxNode } from "@lezer/common";
type apiCompleteResult = {
  completions: Array<{ label: string; detail?: string; quoted: boolean }>;
};

export const complete = async (ctx: CompletionContext) => {
  const tree = syntaxTree(ctx.state);

  const completion: CompletionResult = {
    from: ctx.pos,
    filter: false,
    options: [],
  };

  // Remote completion
  const remote = async (
    payload: { what: string; column?: string; prefix?: string },
    transform = (x: { label: string; detail?: string }) => x
  ) => {
    const response = await fetch("/api/v0/console/filter/complete", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    if (!response.ok) return;
    const data: apiCompleteResult = await response.json();
    completion.options = [
      ...completion.options,
      ...(data.completions ?? []).map(({ label, detail, quoted }) =>
        transform({
          label: quoted ? `"${label}"` : label,
          detail,
        })
      ),
    ];
  };

  // Some helpers to match nodes.
  const nodeAncestor = (node: SyntaxNode | null, names: string[]) => {
    for (let n = node; n; n = n.parent) {
      if (names.includes(n.name)) {
        return n;
      }
    }
    return null;
  };
  const nodePrevSibling = (node: SyntaxNode | null) => {
    for (let n = node?.prevSibling; n; n = n.prevSibling) {
      if (!["LineComment", "BlockComment"].includes(n.name)) {
        return n;
      }
    }
    return null;
  };
  const nodeRightMostChildBefore = (node: SyntaxNode | null, pos: number) => {
    // Go to the right most child
    let n = node;
    for (;;) {
      if (!n) break;
      if (!n.lastChild) {
        return n;
      }
      n = n.lastChild;
      while (n && n.to > pos) {
        n = n.prevSibling;
      }
    }
    return null;
  };

  let nodeBefore: SyntaxNode | null = tree.resolve(ctx.pos, -1);
  let n: SyntaxNode | null = null;
  if (["LineComment", "BlockComment"].includes(nodeBefore.name)) {
    // Do not complete !
  } else if ((n = nodeAncestor(nodeBefore, ["Column"]))) {
    completion.from = n.from;
    completion.to = n.to;
    await remote({
      what: "column",
      prefix: ctx.state.sliceDoc(n.from, n.to),
    });
  } else if ((n = nodeAncestor(nodeBefore, ["Operator"]))) {
    const c = nodePrevSibling(n);
    if (c?.name === "Column") {
      completion.from = n.from;
      completion.to = n.to;
      await remote({
        what: "operator",
        column: ctx.state.sliceDoc(c.from, c.to),
        prefix: ctx.state.sliceDoc(n.from, n.to),
      });
    }
  } else if (
    nodeBefore.name !== "ValueRParen" &&
    (n = nodeAncestor(nodeBefore, ["Value"]))
  ) {
    const c = nodePrevSibling(nodePrevSibling(n));
    if (c?.name === "Column") {
      let prefix: string | undefined = ctx.state.sliceDoc(
        nodeBefore.from,
        nodeBefore.to
      );
      completion.from = nodeBefore.from;
      completion.to = nodeBefore.to;
      if (
        ["ValueLParen", "ValueComma", "ListOfValues"].includes(nodeBefore.name)
      ) {
        // Empty term
        prefix = undefined;
        completion.from = ctx.pos;
        completion.to = undefined;
      } else if (nodeBefore.name === "String") {
        prefix = prefix.replace(/^["']/, "").replace(/["']$/, "");
      }
      await remote(
        {
          what: "value",
          column: ctx.state.sliceDoc(c.from, c.to),
          prefix: prefix,
        },
        (o) =>
          nodeAncestor(nodeBefore, ["ListOfValues", "ValueLParen"])
            ? { ...o, apply: `${o.label},` }
            : o
      );
    }
  } else if (nodeBefore.name === "Filter" || nodeBefore.name === "⚠") {
    // We are on a whitespace or on something we don't know
    nodeBefore = nodeRightMostChildBefore(tree.topNode, ctx.pos);
    if (nodeBefore?.name === "⚠") {
      completion.from = nodeBefore.from;
      completion.to = nodeBefore.to;
      nodeBefore = nodeBefore.prevSibling;
    }
    if ((n = nodeAncestor(nodeBefore, ["Column"]))) {
      await remote({
        what: "operator",
        column: ctx.state.sliceDoc(n.from, n.to),
      });
    } else if ((n = nodeAncestor(nodeBefore, ["Operator"]))) {
      const c = nodePrevSibling(n);
      if (c?.name === "Column") {
        await remote({
          what: "value",
          column: ctx.state.sliceDoc(c.from, c.to),
        });
      }
    } else if ((n = nodeAncestor(nodeBefore, ["Value"]))) {
      completion.options = [
        { label: "AND", detail: "logic operator" },
        { label: "OR", detail: "logic operator" },
        { label: "AND NOT", detail: "logic operator" },
        { label: "OR NOT", detail: "logic operator" },
        ...completion.options,
      ];
    } else if ((n = nodeAncestor(nodeBefore, ["Or", "And", "Not"]))) {
      if (n.name !== "Not") {
        completion.options = [
          ...completion.options,
          {
            label: "NOT",
            detail: "logic operator",
          },
        ];
      }
      await remote({ what: "column" });
    }
  }

  completion.options.forEach((option) => {
    const from = completion.from;
    option.apply = (option.apply as string) ?? option.label;
    // Insert space before if no space or "("
    if (
      completion.from > 0 &&
      !/[\s(]/.test(ctx.state.sliceDoc(from - 1, from))
    ) {
      option.apply = " " + option.apply;
    }
    // Insert space after if not ending with "("
    if (!option.apply.endsWith("(")) {
      option.apply += " ";
    }
  });
  return completion;
};
