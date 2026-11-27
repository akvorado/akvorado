<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <v-chart :option="option" @click="selectItem" />
</template>

<script lang="ts" setup>
import { inject, computed } from "vue";
import { formatXps, dataColor, dataColorGrey, reverseDimension } from "@/utils";
import { ThemeKey } from "@/components/ThemeProvider.vue";
import { ServerConfigKey } from "@/components/ServerConfigProvider.vue";
import type { GraphSankeyHandlerResult } from ".";
import type { RowSelection } from "./RowActionsMenu.vue";
import { use, type ComposeOption } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { SankeyChart, type SankeySeriesOption } from "echarts/charts";
import {
  TooltipComponent,
  type TooltipComponentOption,
} from "echarts/components";
import type { TooltipCallbackDataParams } from "echarts/types/src/component/tooltip/TooltipView.d.ts";
import VChart from "vue-echarts";
use([CanvasRenderer, SankeyChart, TooltipComponent]);
type ECOption = ComposeOption<SankeySeriesOption | TooltipComponentOption>;

const props = defineProps<{
  data: GraphSankeyHandlerResult;
}>();
const emit = defineEmits<{
  select: [selection: RowSelection];
}>();

const serverConfiguration = inject(ServerConfigKey)!;

// Node IDs are "Dimension: value". A link selects the values of its two nodes.
const selectItem = (params: unknown) => {
  const { seriesIndex, dataType, data, event } = params as {
    seriesIndex?: number;
    dataType?: string;
    data?: { id?: string; source?: string; target?: string };
    event?: { event: MouseEvent };
  };
  if (!event || !data) return;
  // The second half of a bidirectional graph is labeled with reversed
  // dimensions, but the filter applies to the forward direction.
  const reverse = props.data?.bidirectional && seriesIndex === 1;
  const known = serverConfiguration.value?.dimensions ?? [];
  const ids =
    dataType === "node"
      ? [data.id]
      : dataType === "edge"
        ? [data.source, data.target]
        : [];
  const dimensions: string[] = [],
    values: string[] = [];
  for (const id of ids) {
    if (!id) return;
    const [dimension, ...rest] = id.split(": ");
    const value = rest.join(": ");
    if (value === "Other") return;
    dimensions.push(reverse ? reverseDimension(dimension, known) : dimension);
    values.push(value);
  }
  if (dimensions.length === 0) return;
  emit("select", {
    dimensions,
    values,
    x: event.event.clientX,
    y: event.event.clientY,
  });
};

const { isDark } = inject(ThemeKey)!;

// Graph component
const option = computed((): ECOption => {
  const theme = isDark.value ? "dark" : "light";
  const data = props.data || {};
  if (!data.xps) return {};
  // Allocate colors by node value (the part after ": ") so the same value
  // gets the same color across the forward and reverse halves.
  const colorByValue = new Map<string, number>();
  const greyByValue = new Map<string, number>();
  const makeNode = ({ name }: GraphSankeyHandlerResult["nodes"][0]) => {
    const label = name.split(": ").slice(1).join(": ");
    const isGrey = name.endsWith(" Other");
    const cache = isGrey ? greyByValue : colorByValue;
    let idx = cache.get(label);
    if (idx === undefined) {
      idx = cache.size;
      cache.set(label, idx);
    }
    return {
      id: name,
      name: label,
      itemStyle: {
        color: isGrey
          ? dataColorGrey(idx, false, theme)
          : dataColor(idx, false, theme),
      },
    };
  };
  const makeLink = ({
    source,
    target,
    xps,
  }: GraphSankeyHandlerResult["links"][0]) => ({
    source,
    target,
    value: xps,
  });
  const seriesBase = {
    type: "sankey" as const,
    animationDuration: 500,
    emphasis: { focus: "trajectory" as const },
    label: { formatter: "{b}" },
    lineStyle: { color: "gradient" as const, curveness: 0.5 },
  };
  const tooltip: TooltipComponentOption = {
    confine: true,
    trigger: "item",
    triggerOn: "mousemove",
    formatter(params) {
      if (Array.isArray(params)) return "";
      const { dataType, marker, data, value } =
        params as TooltipCallbackDataParams;
      if (dataType === "node") {
        const nodeData = data as NonNullable<SankeySeriesOption["nodes"]>[0];
        return [
          marker,
          `<span style="display:inline-block;margin-left:1em;">${nodeData.name}</span>`,
          `<span style="display:inline-block;margin-left:2em;font-weight:bold;">${formatXps(
            (value?.valueOf() as number) ?? 0,
          )}`,
        ].join("");
      } else if (dataType === "edge") {
        const edgeData = data as NonNullable<SankeySeriesOption["edges"]>[0];
        const source =
          edgeData.source?.toString().split(": ").slice(1).join(": ") ?? "???";
        const target =
          edgeData.target?.toString().split(": ").slice(1).join(": ") ?? "???";
        return value
          ? [
              `${source} → ${target}`,
              `<span style="display:inline-block;margin-left:2em;font-weight:bold;">${formatXps(
                value.valueOf() as number,
              )}`,
            ].join("")
          : "";
      }
      return "";
    },
    valueFormatter: (value) => formatXps((value?.valueOf() as number) ?? 0),
  };

  // Put the labels of the last column of nodes on their left. The nodes on the
  // last column are not the source of a link.
  const positionLabels = <T extends { id: string }>(
    nodes: T[],
    links: { source: string }[],
  ) => {
    const sources = new Set(links.map((l) => l.source));
    return nodes.map((n) =>
      sources.has(n.id) ? n : { ...n, label: { position: "left" as const } },
    );
  };

  if (data.bidirectional) {
    const fwdLinks = data.links.filter((l) => l.axis === 1).map(makeLink);
    // Flip source/target on the reverse half so the column order is
    // mirrored: the dimension closest to the forward side appears first.
    const revLinks = data.links
      .filter((l) => l.axis === 2)
      .map(makeLink)
      .map((l) => ({ ...l, source: l.target, target: l.source }));
    const fwdNodes = positionLabels(
      data.nodes.filter((n) => n.axis === 1).map(makeNode),
      fwdLinks,
    );
    const revNodes = positionLabels(
      data.nodes.filter((n) => n.axis === 2).map(makeNode),
      revLinks,
    );
    return {
      backgroundColor: "transparent",
      tooltip,
      series: [
        {
          ...seriesBase,
          left: "1%",
          right: "50%",
          data: fwdNodes,
          links: fwdLinks,
        },
        {
          ...seriesBase,
          left: "50%",
          right: "1%",
          data: revNodes,
          links: revLinks,
        },
      ],
    };
  }

  const links = data.links.map(makeLink);
  const nodes = positionLabels(data.nodes.map(makeNode), links);
  return {
    backgroundColor: "transparent",
    tooltip,
    series: [
      {
        ...seriesBase,
        left: "1%",
        right: "1%",
        data: nodes,
        links,
      },
    ],
  };
});
</script>
