// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import { parser } from "./syntax.grammar";
import { fileTests } from "@lezer/generator/dist/test";
import { describe, it } from "vitest";

import * as fs from "fs";
import * as path from "path";
import { fileURLToPath } from "url";
const caseFile = path.join(
  path.dirname(fileURLToPath(import.meta.url)),
  "grammar.test.txt"
);

describe("filter parsing", () => {
  for (const { name, run } of fileTests(
    fs.readFileSync(caseFile, "utf8"),
    "grammar.test.txt"
  ))
    it(name, () => run(parser));
});
