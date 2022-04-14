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
      <div>
        <h2
          class="title-font text-3xl font-medium text-gray-900 dark:text-gray-200"
        >
          {{ flows }}
        </h2>
        <p class="leading-relaxed">Flows/s</p>
      </div>
      <div>
        <h2
          class="title-font text-3xl font-medium text-gray-900 dark:text-gray-200"
        >
          {{ exporters }}
        </h2>
        <p class="leading-relaxed">Exporters</p>
      </div>
      <div class="col-span-2 row-span-3 text-left">
        <h1 class="font-semibold leading-relaxed">Last flow</h1>
        <table class="w-full table-fixed text-sm">
          <tbody>
            <tr v-for="[field, value] in lastFlow" :key="field">
              <td class="w-2/5 overflow-hidden text-ellipsis pr-3">
                {{ field }}
              </td>
              <td class="w-3/5 overflow-hidden text-ellipsis">
                {{ value }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onBeforeUnmount } from "vue";
const flows = ref("???");
const exporters = ref("???");
const lastFlow = ref({});
const refreshInterval = 10_000;

const fetchData = async () => {
  const responseOK = (r) => {
    if (!r.ok) {
      throw new Error(`got a ${r.status} error`);
    }
    return true;
  };
  const [dataFlows, dataExporters, dataLastFlow] = await Promise.allSettled([
    fetch("/api/v0/console/widget/flow-rate")
      .then((r) => responseOK(r) && r.json())
      .then((d) => d.rate),
    fetch("/api/v0/console/widget/exporters")
      .then((r) => responseOK(r) && r.json())
      .then((d) => d.exporters.length),
    fetch("/api/v0/console/widget/flow-last").then(
      (r) => responseOK(r) && r.json()
    ),
  ]);

  flows.value = exporters.value = "???";
  if (dataFlows.status === "fulfilled") {
    if (dataFlows.value > 1_500_000) {
      flows.value = (dataFlows.value / 1_000_000).toFixed(1) + "M";
    } else if (dataFlows.value > 1_500) {
      flows.value = (dataFlows.value / 1_000).toFixed(1) + "K";
    } else {
      flows.value = dataFlows.value.toFixed(0);
    }
  }
  if (dataExporters.status === "fulfilled") {
    exporters.value = dataExporters.value;
  }

  lastFlow.value = {};
  if (dataLastFlow.status === "fulfilled") {
    // Sort fields
    lastFlow.value = Object.entries(dataLastFlow.value).sort(([f1], [f2]) => {
      const metric = {
        Dat: 1,
        Tim: 2,
        Byt: 3,
        Pac: 4,
        ETy: 5,
        Pro: 6,
        Exp: 7,
        Sam: 8,
        Seq: 9,
        Dst: 10,
        Src: 10,
        InI: 11,
        Out: 11,
      };
      const m1 = metric[f1.substring(0, 3)] || 100;
      const m2 = metric[f2.substring(0, 3)] || 100;
      const cmp = m1 - m2;
      if (cmp) {
        return cmp;
      }
      if (m1 === 10) {
        f1 = f1.substring(3);
        f2 = f2.substring(3);
      } else if (m1 === 11) {
        if (f1.startsWith("InIf")) {
          f1 = f1.substring(4);
        } else {
          f1 = f1.substring(5);
        }
        if (f2.startsWith("InIf")) {
          f2 = f2.substring(4);
        } else {
          f2 = f2.substring(5);
        }
      }
      return f1.localeCompare(f2);
    });
  }
  timer = setTimeout(fetchData, refreshInterval);
};

let timer = 0;
fetchData();
onBeforeUnmount(() => clearTimeout(timer));
</script>
