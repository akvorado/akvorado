<template>
  <div class="container mx-auto p-5">
    <div
      class="grid grid-cols-2 gap-4 text-center md:grid-cols-4 xl:grid-cols-6"
    >
      <div class="col-span-2 flex flex-row">
        <div class="my-auto mr-4 basis-1/3">
          <div class="h-24 w-24">
            <img src="@/assets/images/akvorado.svg" />
          </div>
        </div>
        <div class="grow">
          <p class="leading-relaxed">
            <strong>Akvorado</strong> is a flow collector, hydrater and
            exporter. It receives flows, adds some data like interface names and
            countries, and exports them to Kafka.
          </p>
        </div>
      </div>
      <WidgetFlowRate :refresh="refreshOften" class="rounded-md p-4 shadow" />
      <WidgetExporters
        :refresh="refreshOccasionally"
        class="rounded-md p-4 shadow"
      />
      <WidgetLastFlow
        :refresh="refreshOften"
        class="order-last col-span-2 row-span-3 xl:order-none"
      />
      <WidgetTop what="src-as" title="Top AS" :refresh="refreshOccasionally" />
      <WidgetTop
        what="src-port"
        title="Top ports"
        :refresh="refreshOccasionally"
      />
      <WidgetTop
        what="protocol"
        title="Top protocols"
        :refresh="refreshOccasionally"
      />
      <WidgetTop
        what="src-country"
        title="Top countries"
        :refresh="refreshOccasionally"
      />
      <WidgetTop
        what="etype"
        title="IPv4/IPv6"
        :refresh="refreshOccasionally"
      />
      <WidgetGraph
        :refresh="refreshInfrequently"
        class="col-span-2 md:col-span-3"
      />
    </div>
  </div>
</template>

<script setup>
import { useInterval } from "@vueuse/core";
import WidgetLastFlow from "./HomePage/WidgetLastFlow.vue";
import WidgetFlowRate from "./HomePage/WidgetFlowRate.vue";
import WidgetExporters from "./HomePage/WidgetExporters.vue";
import WidgetTop from "./HomePage/WidgetTop.vue";
import WidgetGraph from "./HomePage/WidgetGraph.vue";

const refreshOften = useInterval(10_000);
const refreshOccasionally = useInterval(60_000);
const refreshInfrequently = useInterval(600_000);
</script>
