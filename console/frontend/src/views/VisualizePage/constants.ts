// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

export const graphTypes = {
  stacked: "Stacked areas",
  lines: "Lines",
  grid: "Grid",
  sankey: "Sankey",
} as const;
export type GraphType = keyof typeof graphTypes;
