<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <v-chart :option="graph" />
</template>

<script setup>
const props = defineProps({
  data: {
    type: Object,
    default: null,
  },
});

import { inject, computed } from "vue";
import { formatXps, dataColor, dataColorGrey } from "@/utils";
const { isDark } = inject("theme");

import { use } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { SankeyChart } from "echarts/charts";
import { TooltipComponent } from "echarts/components";
import VChart from "vue-echarts";
use([CanvasRenderer, SankeyChart, TooltipComponent]);

// Graph component
const graph = computed(() => {
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
      formatter({ dataType, marker, data, value }) {
        if (dataType === "node") {
          return [
            marker,
            `<span style="display:inline-block;margin-left:1em;">${data.name}</span>`,
            `<span style="display:inline-block;margin-left:2em;font-weight:bold;">${formatXps(
              value
            )}`,
          ].join("");
        } else if (dataType === "edge") {
          const source = data.source.split(": ").slice(1).join(": ");
          const target = data.target.split(": ").slice(1).join(": ");
          return [
            `${source} → ${target}`,
            `<span style="display:inline-block;margin-left:2em;font-weight:bold;">${formatXps(
              data.value
            )}`,
          ].join("");
        }
      },
      valueFormatter: formatXps,
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
