// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

export function formatXps(value: number) {
  value = Math.abs(value);
  const suffixes = ["", "K", "M", "G", "T"];
  let idx = 0;
  while (value >= 1000 && idx < suffixes.length) {
    value /= 1000;
    idx++;
  }
  return `${value.toFixed(2)}${suffixes[idx]}`;
}

// Order function for field names
export function compareFields(f1: string, f2: string) {
  const metric: { [prefix: string]: number } = {
    Dat: 1,
    Tim: 2,
    Byt: 3,
    Pac: 4,
    Exp: 7,
    Sam: 8,
    Seq: 9,
    Src: 10,
    Dst: 12,
    InI: 11,
    Out: 13,
  };
  const m1 = metric[f1.substring(0, 3)] || 100;
  const m2 = metric[f2.substring(0, 3)] || 100;
  const cmp = m1 - m2;
  if (cmp) {
    return cmp;
  }
  if (m1 === 10) {
    f1 = f1.substring(3);
    f2 = f2.substring(3);
  } else if (m1 === 11) {
    if (f1.startsWith("InIf")) {
      f1 = f1.substring(4);
    } else {
      f1 = f1.substring(5);
    }
    if (f2.startsWith("InIf")) {
      f2 = f2.substring(4);
    } else {
      f2 = f2.substring(5);
    }
  }
  return f1.localeCompare(f2);
}

export { dataColor, dataColorGrey } from "./palette.js";
