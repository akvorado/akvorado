<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div class="text-left">
    <h1 class="font-semibold leading-relaxed">Last flow</h1>
    <table class="w-full max-w-md text-sm">
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

<script lang="ts" setup>
import { computed } from "vue";
import { useFetch } from "@vueuse/core";
import { compareFields } from "../../utils";

const props = withDefaults(
  defineProps<{
    refresh?: number;
  }>(),
  { refresh: 0 },
);

const url = computed(() => `/api/v0/console/widget/flow-last?${props.refresh}`);
const { data } = useFetch(url, { refetch: true })
  .get()
  .json<Record<string, string | number>>();
const lastFlow = computed((): [string, string | number][] => ({
  ...Object.entries(data.value || {}).sort(([f1], [f2]) =>
    compareFields(f1, f2),
  ),
}));
</script>
