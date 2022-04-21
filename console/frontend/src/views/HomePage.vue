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
      <WidgetFlowRate :refresh="refreshOften" class="rounded-md p-4 shadow" />
      <WidgetExporters :refresh="refreshOften" class="rounded-md p-4 shadow" />
      <WidgetLastFlow
        :refresh="refreshOften"
        class="order-last col-span-2 row-span-3 xl:order-none"
      />
      <WidgetTopSrcAS :refresh="refreshOccasionally" />
    </div>
  </div>
</template>

<script setup>
import { ref, onBeforeUnmount } from "vue";
import WidgetLastFlow from "../components/WidgetLastFlow.vue";
import WidgetFlowRate from "../components/WidgetFlowRate.vue";
import WidgetExporters from "../components/WidgetExporters.vue";
import WidgetTopSrcAS from "../components/WidgetTopSrcAS.vue";

const refreshOften = ref(0);
const refreshOccasionally = ref(0);
let timerOften = setInterval(() => refreshOften.value++, 10_000);
let timerOccasionally = setInterval(() => refreshOccasionally.value++, 60_000);
onBeforeUnmount(() => {
  clearInterval(timerOften);
  clearInterval(timerOccasionally);
});
</script>
