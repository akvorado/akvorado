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
    lastFlow.value = Object.entries(data).sort(([f1], [f2]) => {
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
  },
  { immediate: true }
);
</script>
