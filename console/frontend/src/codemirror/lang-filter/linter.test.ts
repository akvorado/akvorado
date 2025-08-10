// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { EditorState } from "@codemirror/state";
import type { EditorView } from "@codemirror/view";
import { linterSource } from "./linter";
import { filterLanguage } from ".";

function createEditorView(doc: string) {
  const state = EditorState.create({
    doc,
    extensions: [filterLanguage()],
  });
  return { state } as EditorView;
}

describe("linter", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn((_: string, options: RequestInit) => {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(JSON.parse(options.body!.toString())),
        });
      }),
    );
  });

  it("returns empty array for successful validation", async () => {
    const view = createEditorView(
      "InIfBoundary = 'external' AND InIfProvider = 'cogent'",
    );

    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          message: "ok",
          parsed: "InIfBoundary = 'external' AND InIfProvider = 'cogent'",
        }),
    } as Response);

    const diagnostics = await linterSource(view);
    expect(fetch).toHaveBeenCalledWith("/api/v0/console/filter/validate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        filter: "InIfBoundary = 'external' AND InIfProvider = 'cogent'",
      }),
    });
    expect(diagnostics).toEqual([]);
  });

  it("returns diagnostics for validation errors", async () => {
    const view = createEditorView(
      "InIfBoundary = 'external' AND InIfProvider = cogent'",
    );

    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          message:
            'at line 1, position 44: no match found, expected: "\'", "--", "/*", "\\"" or [ \\n\\r\\t]',
          errors: [
            {
              message:
                'no match found, expected: "\'", "--", "/*", "\\"" or [ \\n\\r\\t]',
              line: 1,
              column: 44,
              offset: 43,
            },
          ],
        }),
    } as Response);

    const diagnostics = await linterSource(view);
    expect(diagnostics).toEqual([
      {
        from: 43,
        to: 44,
        severity: "error",
        message:
          'no match found, expected: "\'", "--", "/*", "\\"" or [ \\n\\r\\t]',
      },
    ]);
  });

  it("handles response with no errors field", async () => {
    const view = createEditorView("valid filter");

    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          message: "ok",
        }),
    } as Response);

    const diagnostics = await linterSource(view);
    expect(diagnostics).toEqual([]);
  });

  it("returns empty array when fetch fails", async () => {
    const view = createEditorView("some filter");

    vi.mocked(fetch).mockResolvedValueOnce({
      ok: false,
    } as Response);

    const diagnostics = await linterSource(view);
    expect(diagnostics).toEqual([]);
  });
});
