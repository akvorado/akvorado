// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import type { GraphType } from "./graphtypes";

export type Units = "l3bps" | "l2bps" | "pps" | "inl2%" | "outl2%";
export type GraphSankeyHandlerInput = {
  start: string;
  end: string;
  dimensions: string[];
  limit: number;
  filter: string;
  units: Units;
};
export type GraphLineHandlerInput = GraphSankeyHandlerInput & {
  points: number;
  bidirectional: boolean;
  "previous-period": boolean;
};
export type GraphSankeyHandlerOutput = {
  rows: string[][];
  xps: number[];
  nodes: string[];
  links: {
    source: string;
    target: string;
    xps: number;
  }[];
};
export type GraphLineHandlerOutput = {
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
export type GraphSankeyHandlerResult = GraphSankeyHandlerOutput & {
  graphType: Extract<GraphType, "sankey">;
} & Pick<GraphSankeyHandlerInput, "start" | "end" | "dimensions" | "units">;
export type GraphLineHandlerResult = GraphLineHandlerOutput & {
  graphType: Exclude<GraphType, "sankey">;
} & Pick<
    GraphLineHandlerInput,
    "start" | "end" | "dimensions" | "units" | "bidirectional"
  >;
