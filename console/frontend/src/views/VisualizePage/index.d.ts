// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import type { GraphType } from "./constants";

export type Units = "l3bps" | "l2bps" | "pps";
export type SankeyHandlerInput = {
  start: string;
  end: string;
  dimensions: string[];
  limit: number;
  filter: string;
  units: Units;
};
export type GraphHandlerInput = SankeyHandlerInput & {
  points: number;
  bidirectional: boolean;
  "previous-period": boolean;
};
export type SankeyHandlerOutput = {
  rows: string[][];
  xps: number[];
  nodes: string[];
  links: {
    source: string;
    target: string;
    xps: number;
  }[];
};
export type GraphHandlerOutput = {
  t: string[];
  rows: string[][];
  points: number[][];
  axis: number[];
  "axis-names": Record<number, string>;
  average: number[];
  min: number[];
  max: number[];
  "95th": number[];
};
export type SankeyHandlerResult = SankeyHandlerOutput & {
  graphType: Extract<GraphType, "sankey">;
} & Pick<SankeyHandlerInput, "start" | "end" | "dimensions" | "units">;
export type GraphHandlerResult = GraphHandlerOutput & {
  graphType: Exclude<GraphType, "sankey">;
} & Pick<
    GraphHandlerInput,
    "start" | "end" | "dimensions" | "units" | "bidirectional"
  >;
