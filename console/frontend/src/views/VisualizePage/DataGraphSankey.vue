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
import type { TooltipCallbackDataParams } from "echarts/types/src/component/tooltip/TooltipView";
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
  return {
    backgroundColor: "transparent",
    tooltip: {
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
              value.valueOf() as number,
            )}`,
          ].join("");
        } else if (dataType === "edge") {
          const edgeData = data as NonNullable<SankeySeriesOption["edges"]>[0];
          const source =
            edgeData.source?.toString().split(": ").slice(1).join(": ") ??
            "???";
          const target =
            edgeData.target?.toString().split(": ").slice(1).join(": ") ??
            "???";
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
      valueFormatter: (value) => formatXps(value.valueOf() as number),
    },
    series: [
      {
        type: "sankey",
        animationDuration: 500,
        emphasis: {
          focus: "adjacency",
        },
        data: data.nodes.map((v) => ({
          id: v,
          name: v.split(": ").slice(1).join(": "),
          itemStyle: {
            color: v.endsWith(" Other")
              ? dataColorGrey(greyNodes++, false, theme)
              : dataColor(colorNodes++, false, theme),
          },
        })),
        links: data.links.map(({ source, target, xps }) => ({
          source,
          target,
          value: xps,
        })),
        label: {
          formatter: "{b}",
        },
        lineStyle: {
          color: "gradient",
          curveness: 0.5,
        },
      },
    ],
  };
});
</script>
