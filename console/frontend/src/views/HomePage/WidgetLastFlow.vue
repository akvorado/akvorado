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
import { ref, watch } from "vue";
import { compareFields } from "../../utils";

const props = defineProps({
  refresh: {
    type: Number,
    default: 0,
  },
});
const lastFlow = ref({});

watch(
  () => props.refresh,
  async () => {
    const response = await fetch("/api/v0/console/widget/flow-last");
    if (!response.ok) {
      // Just don't update component.
      return;
    }
    const data = await response.json();
    lastFlow.value = Object.entries(data).sort(([f1], [f2]) =>
      compareFields(f1, f2)
    );
  },
  { immediate: true }
);
</script>
