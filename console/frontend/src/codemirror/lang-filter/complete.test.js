import { EditorState } from "@codemirror/state";
import { CompletionContext, autocompletion } from "@codemirror/autocomplete";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { filterLanguage, filterCompletion } from ".";

async function get(doc) {
  let cur = doc.indexOf("|");
  doc = doc.slice(0, cur) + doc.slice(cur + 1);
  let state = EditorState.create({
    doc,
    selection: { anchor: cur },
    extensions: [filterLanguage(), filterCompletion(), autocompletion()],
  });
  return await state.languageDataAt("autocomplete", cur)[0](
    new CompletionContext(state, cur, true)
  );
}

describe("filter completion", () => {
  let fetchOptions = {};
  afterEach(() => {
    vi.restoreAllMocks();
    fetchOptions = {};
  });
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn((url, options) => {
        fetchOptions = options;
        const body = JSON.parse(options.body);
        return {
          ok: true,
          async json() {
            switch (body.what) {
              case "column":
                return {
                  completions: [
                    { label: "SrcAS", detail: "column name", quoted: false },
                    { label: "SrcAddr", detail: "column name", quoted: false },
                    {
                      label: "SrcCountry",
                      detail: "column name",
                      quoted: false,
                    },
                    { label: "DstAS", detail: "column name", quoted: false },
                    { label: "DstAddr", detail: "column name", quoted: false },
                    {
                      label: "DstCountry",
                      detail: "column name",
                      quoted: false,
                    },
                  ].filter(({ label }) => label.startsWith(body.prefix ?? "")),
                };
              case "operator":
                switch (body.column) {
                  case "SrcAS":
                    return {
                      completions: [
                        { label: "=", detail: "operator", quoted: false },
                        { label: "!=", detail: "operator", quoted: false },
                        { label: "IN", detail: "operator", quoted: false },
                      ].filter(({ label }) =>
                        label.startsWith(body.prefix ?? "")
                      ),
                    };
                  default:
                    throw new Error(`unhandled column name: ${body.column}`);
                }
              case "value":
                switch (body.column) {
                  case "DstNetName":
                    return {
                      completions: [
                        {
                          label: "something",
                          detail: "network name",
                          quoted: "true",
                        },
                        {
                          label: "squid",
                          detail: "network name",
                          quoted: "true",
                        },
                      ].filter(({ label }) =>
                        label.startsWith(body.prefix ?? "")
                      ),
                    };

                  case "SrcAS":
                    return {
                      completions: [
                        {
                          label: "AS65403",
                          detail: "AS number",
                          quoted: false,
                        },
                        {
                          label: "AS65404",
                          detail: "AS number",
                          quoted: false,
                        },
                        {
                          label: "AS65405",
                          detail: "AS number",
                          quoted: false,
                        },
                      ],
                    };
                  default:
                    throw new Error(`unhandled column name: ${body.column}`);
                }
              default:
                throw new Error(`unhandled what: ${body.what}`);
            }
          },
        };
      })
    );
  });

  it("completes column names", async () => {
    let { from, to, options } = await get("S|");
    expect(fetchOptions.method).toEqual("POST");
    expect(JSON.parse(fetchOptions.body)).toEqual({
      what: "column",
      prefix: "S",
    });
    expect({ from, to, options }).toEqual({
      from: 0,
      to: 1,
      options: [
        { apply: "SrcAS ", detail: "column name", label: "SrcAS" },
        { apply: "SrcAddr ", detail: "column name", label: "SrcAddr" },
        { apply: "SrcCountry ", detail: "column name", label: "SrcCountry" },
      ],
    });
  });

  it("completes inside column names", async () => {
    let { from, to, options } = await get("S|rc =");
    expect(fetchOptions.method).toEqual("POST");
    expect(JSON.parse(fetchOptions.body)).toEqual({
      what: "column",
      prefix: "Src",
    });
    expect({ from, to, options }).toEqual({
      from: 0,
      to: 3,
      options: [
        { apply: "SrcAS ", detail: "column name", label: "SrcAS" },
        { apply: "SrcAddr ", detail: "column name", label: "SrcAddr" },
        { apply: "SrcCountry ", detail: "column name", label: "SrcCountry" },
      ],
    });
  });

  it("completes operator names", async () => {
    let { from, to, options } = await get("SrcAS |");
    expect(JSON.parse(fetchOptions.body)).toEqual({
      what: "operator",
      column: "SrcAS",
    });
    expect({ from, to, options }).toEqual({
      from: 5,
      to: 5,
      options: [
        { apply: " = ", detail: "operator", label: "=" },
        { apply: " != ", detail: "operator", label: "!=" },
        { apply: " IN ", detail: "operator", label: "IN" },
      ],
    });
  });

  it("completes values", async () => {
    let { from, to, options } = await get("SrcAS = fac|");
    expect(JSON.parse(fetchOptions.body)).toEqual({
      what: "value",
      column: "SrcAS",
      prefix: "fac",
    });
    expect({ from, to, options }).toEqual({
      from: 8,
      to: 11,
      options: [
        { apply: "AS65403 ", detail: "AS number", label: "AS65403" },
        { apply: "AS65404 ", detail: "AS number", label: "AS65404" },
        { apply: "AS65405 ", detail: "AS number", label: "AS65405" },
      ],
    });
  });

  it("completes quoted values", async () => {
    let { from, to, options } = await get('DstNetName = "so|');
    expect(JSON.parse(fetchOptions.body)).toEqual({
      what: "value",
      column: "DstNetName",
      prefix: "so",
    });
    expect({ from, to, options }).toEqual({
      from: 13,
      to: 16,
      options: [
        { apply: '"something" ', detail: "network name", label: '"something"' },
      ],
    });
  });

  it("completes quoted values even when not quoted", async () => {
    let { from, to, options } = await get("DstNetName = so|");
    expect(JSON.parse(fetchOptions.body)).toEqual({
      what: "value",
      column: "DstNetName",
      prefix: "so",
    });
    expect({ from, to, options }).toEqual({
      from: 13,
      to: 15,
      options: [
        { apply: '"something" ', detail: "network name", label: '"something"' },
      ],
    });
  });

  it("completes logic operator", async () => {
    let { from, to, options } = await get("SrcAS = 1000 A|");
    expect(fetchOptions).toEqual({});
    expect({ from, to, options }).toEqual({
      from: 13,
      to: 14,
      options: [
        { apply: "AND ", detail: "logic operator", label: "AND" },
        { apply: "OR ", detail: "logic operator", label: "OR" },
        { apply: "AND NOT ", detail: "logic operator", label: "AND NOT" },
        { apply: "OR NOT ", detail: "logic operator", label: "OR NOT" },
      ],
    });
  });

  it("does not complete comments", async () => {
    let { from, to, options } = await get("SrcAS = 1000 -- h|");
    expect(fetchOptions).toEqual({});
    expect({ from, to, options }).toEqual({
      from: 17,
      to: undefined,
      options: [],
    });
  });

  it("completes inside operator", async () => {
    let { from, to, options } = await get("SrcAS I|");
    expect(JSON.parse(fetchOptions.body)).toEqual({
      what: "operator",
      prefix: "I",
      column: "SrcAS",
    });
    expect({ from, to, options }).toEqual({
      from: 6,
      to: 7,
      options: [{ apply: "IN ", detail: "operator", label: "IN" }],
    });
  });

  it("completes empty list of values", async () => {
    let { from, to, options } = await get("SrcAS IN (|");
    expect(JSON.parse(fetchOptions.body)).toEqual({
      what: "value",
      column: "SrcAS",
      prefix: null,
    });
    expect({ from, to, options }).toEqual({
      from: 10,
      to: null,
      options: [
        { apply: "AS65403, ", detail: "AS number", label: "AS65403" },
        { apply: "AS65404, ", detail: "AS number", label: "AS65404" },
        { apply: "AS65405, ", detail: "AS number", label: "AS65405" },
      ],
    });
  });

  it("completes non-empty list of values", async () => {
    let { from, to, options } = await get("SrcAS IN (100,|");
    expect(JSON.parse(fetchOptions.body)).toEqual({
      what: "value",
      column: "SrcAS",
      prefix: null,
    });
    expect({ from, to, options }).toEqual({
      from: 14,
      to: null,
      options: [
        { apply: " AS65403, ", detail: "AS number", label: "AS65403" },
        { apply: " AS65404, ", detail: "AS number", label: "AS65404" },
        { apply: " AS65405, ", detail: "AS number", label: "AS65405" },
      ],
    });
  });

  it("completes NOT", async () => {
    let { from, to, options } = await get("SrcAS = 100 AND |");
    expect(JSON.parse(fetchOptions.body)).toEqual({
      what: "column",
    });
    expect({ from, to, options }).toEqual({
      from: 15,
      to: 15,
      options: [
        { apply: " NOT ", detail: "logic operator", label: "NOT" },
        { apply: " SrcAS ", detail: "column name", label: "SrcAS" },
        { apply: " SrcAddr ", detail: "column name", label: "SrcAddr" },
        { apply: " SrcCountry ", detail: "column name", label: "SrcCountry" },
        { apply: " DstAS ", detail: "column name", label: "DstAS" },
        { apply: " DstAddr ", detail: "column name", label: "DstAddr" },
        { apply: " DstCountry ", detail: "column name", label: "DstCountry" },
      ],
    });
  });

  it("completes column after logic operator", async () => {
    let { from, to, options } = await get("SrcAS = 100 AND S|");
    expect(JSON.parse(fetchOptions.body)).toEqual({
      what: "column",
      prefix: "S",
    });
    expect({ from, to, options }).toEqual({
      from: 16,
      to: 17,
      options: [
        { apply: "SrcAS ", detail: "column name", label: "SrcAS" },
        { apply: "SrcAddr ", detail: "column name", label: "SrcAddr" },
        { apply: "SrcCountry ", detail: "column name", label: "SrcCountry" },
      ],
    });
  });
});
