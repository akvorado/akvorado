<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div class="container mx-auto">
    <div class="grid grid-cols-1 gap-4 p-5 text-center xl:grid-cols-3">
      <div
        class="grid auto-rows-min grid-cols-2 gap-4 md:grid-cols-4 xl:col-span-2"
      >
        <div class="col-span-2 flex flex-row">
          <div class="my-auto mr-4 basis-1/3">
            <div class="h-24 w-24">
              <img src="@/assets/images/akvorado.svg" />
            </div>
          </div>
          <div class="grow">
            <p class="leading-relaxed">
              <strong>Akvorado</strong> is a flow collector, enricher and
              exporter. It receives flows, adds some data like interface names
              and countries, and exports them to Kafka.
            </p>
          </div>
        </div>
        <WidgetFlowRate
          :refresh="refreshOften"
          class="rounded-md p-4 shadow dark:shadow-white/10"
        />
        <WidgetExporters
          :refresh="refreshOccasionally"
          class="rounded-md p-4 shadow dark:shadow-white/10"
        />
        <WidgetTop
          v-for="widget in topWidgets"
          :key="widget"
          :what="widget"
          :title="widgetTitle(widget)"
          :refresh="refreshOccasionally"
        />
        <WidgetGraph
          :refresh="refreshInfrequently"
          class="col-span-2 md:col-span-3"
        />
      </div>
      <WidgetLastFlow :refresh="refreshOften" />
    </div>
  </div>
</template>

<script setup>
import { inject, computed } from "vue";
import { useInterval } from "@vueuse/core";
import WidgetLastFlow from "./HomePage/WidgetLastFlow.vue";
import WidgetFlowRate from "./HomePage/WidgetFlowRate.vue";
import WidgetExporters from "./HomePage/WidgetExporters.vue";
import WidgetTop from "./HomePage/WidgetTop.vue";
import WidgetGraph from "./HomePage/WidgetGraph.vue";

const serverConfiguration = inject("server-configuration");
const topWidgets = computed(() => serverConfiguration.value?.topWidgets ?? []);
const widgetTitle = (name) =>
  ({
    "src-as": "Top source AS",
    "dst-as": "Top destination AS",
    "src-country": "Top source countries",
    "dst-country": "Top destination countries",
    exporter: "Top exporters",
    protocol: "Top protocols",
    etype: "IPv4/IPv6",
    "src-port": "Top source ports",
    "dst-port": "Top destination ports",
  }[name] ?? "???");

const refreshOften = useInterval(10_000);
const refreshOccasionally = useInterval(60_000);
const refreshInfrequently = useInterval(600_000);
</script>
