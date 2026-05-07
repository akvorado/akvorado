<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <v-chart :option="option" />
</template>

<script lang="ts" setup>
import { inject, computed } from "vue";
import { formatXps, dataColor, dataColorGrey } from "@/utils";
import { ThemeKey } from "@/components/ThemeProvider.vue";
import type { GraphSankeyHandlerResult } from ".";
import { use, type ComposeOption } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { SankeyChart, type SankeySeriesOption } from "echarts/charts";
import {
  TooltipComponent,
  type TooltipComponentOption,
  type GridComponentOption,
} from "echarts/components";
import type { TooltipCallbackDataParams } from "echarts/types/src/component/tooltip/TooltipView.d.ts";
import VChart from "vue-echarts";
use([CanvasRenderer, SankeyChart, TooltipComponent]);
type ECOption = ComposeOption<
  SankeySeriesOption | TooltipComponentOption | GridComponentOption
>;

const props = defineProps<{
  data: GraphSankeyHandlerResult;
}>();

const { isDark } = inject(ThemeKey)!;

// Graph component
const option = computed((): ECOption => {
  const theme = isDark.value ? "dark" : "light";
  const data = props.data || {};
  if (!data.xps) return {};
  let greyNodes = 0;
  let colorNodes = 0;
  const makeNode = ({ name }: GraphSankeyHandlerResult["nodes"][0]) => ({
    id: name,
    name: name.split(": ").slice(1).join(": "),
    itemStyle: {
      color: name.endsWith(" Other")
        ? dataColorGrey(greyNodes++, false, theme)
        : dataColor(colorNodes++, false, theme),
    },
  });
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

  if (data.bidirectional) {
    const fwdNodes = data.nodes.filter((n) => n.axis === 1).map(makeNode);
    const revNodes = data.nodes.filter((n) => n.axis === 2).map(makeNode);
    const fwdLinks = data.links.filter((l) => l.axis === 1).map(makeLink);
    const revLinks = data.links.filter((l) => l.axis === 2).map(makeLink);
    return {
      backgroundColor: "transparent",
      tooltip,
      series: [
        {
          ...seriesBase,
          left: "5%",
          right: "55%",
          data: fwdNodes,
          links: fwdLinks,
        },
        {
          ...seriesBase,
          left: "55%",
          right: "5%",
          data: revNodes,
          links: revLinks,
        },
      ],
    };
  }

  return {
    backgroundColor: "transparent",
    tooltip,
    series: [
      {
        ...seriesBase,
        data: data.nodes.map(makeNode),
        links: data.links.map(makeLink),
      },
    ],
  };
});
</script>
