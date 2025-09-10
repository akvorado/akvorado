// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

export function formatXps(value: number) {
  const isNegative = value < 0;
  let absValue = Math.abs(value);
  const suffixes = ["", "K", "M", "G", "T", "P"];
  let idx = 0;
  while (absValue >= 1000 && idx < suffixes.length - 1) {
    absValue /= 1000;
    idx++;
  }
  const sign = isNegative ? "-" : "";
  return `${sign}${absValue.toFixed(2)}${suffixes[idx]}`;
}

// Order function for field names
export function compareFields(f1: string, f2: string) {
  const metric: { [prefix: string]: number } = {
    Tim: 1,
    Byt: 2,
    Pac: 3,
    Exp: 7,
    Sam: 8,
    Src: 10,
    Dst: 12,
    InI: 11,
    Out: 13,
  };
  const m1 = metric[f1.substring(0, 3)] || 100;
  const m2 = metric[f2.substring(0, 3)] || 100;
  const cmp = m1 - m2;
  if (cmp !== 0) {
    return cmp;
  }
  return f1.localeCompare(f2);
}

export { dataColor, dataColorGrey } from "./palette.js";
