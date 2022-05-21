<template>
  <div class="text-left">
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
</template>

<script setup>
const props = defineProps({
  refresh: {
    type: Number,
    default: 0,
  },
});

import { computed } from "vue";
import { useFetch } from "@vueuse/core";
import { compareFields } from "../../utils";

const url = computed(() => "/api/v0/console/widget/flow-last?" + props.refresh);
const { data } = useFetch(url, { refetch: true }).get().json();
const lastFlow = computed(() => ({
  ...(lastFlow.value || {}),
  ...Object.entries(data.value || {}).sort(([f1], [f2]) =>
    compareFields(f1, f2)
  ),
}));
</script>
