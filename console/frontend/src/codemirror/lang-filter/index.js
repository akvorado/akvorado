import { parser } from "./syntax.grammar";
import { LRLanguage, LanguageSupport } from "@codemirror/language";
import { styleTags, tags as t } from "@lezer/highlight";

export const FilterLanguage = LRLanguage.define({
  parser: parser.configure({
    props: [
      styleTags({
        Column: t.propertyName,
        String: t.string,
        Literal: t.literal,
        LineComment: t.lineComment,
        BlockComment: t.blockComment,
        Or: t.logicOperator,
        And: t.logicOperator,
        Not: t.logicOperator,
        Operator: t.compareOperator,
        "( )": t.paren,
      }),
    ],
  }),
  languageData: {
    commentTokens: { line: "--", block: { open: "/*", close: "*/" } },
  },
});

export function filter() {
  return new LanguageSupport(FilterLanguage);
}
