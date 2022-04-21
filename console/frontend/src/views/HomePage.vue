<template>
  <div class="container mx-auto p-5">
    <div
      class="grid grid-cols-2 gap-4 text-center md:grid-cols-4 xl:grid-cols-6"
    >
      <div class="col-span-2 flex flex-row">
        <div class="my-auto mr-4 basis-1/3">
          <div class="h-24 w-24">
            <img src="../assets/images/akvorado.svg" />
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
      <WidgetFlowRate :refresh="refresh" />
      <WidgetExporters :refresh="refresh" />
      <WidgetLastFlow :refresh="refresh" class="col-span-2 row-span-3" />
    </div>
  </div>
</template>

<script setup>
import { ref, onBeforeUnmount } from "vue";
import WidgetLastFlow from "../components/WidgetLastFlow.vue";
import WidgetFlowRate from "../components/WidgetFlowRate.vue";
import WidgetExporters from "../components/WidgetExporters.vue";

const refresh = ref(0);

let timer = setInterval(() => refresh.value++, 10_000);
onBeforeUnmount(() => clearInterval(timer));
</script>
