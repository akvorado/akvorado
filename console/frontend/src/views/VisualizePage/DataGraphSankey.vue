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
          name: v.startsWith("Other ") ? "Other" : v,
          itemStyle: {
            color: v.startsWith("Other ")
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
