<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div class="flex flex-col items-center justify-center">
    <h2
      class="title-font text-3xl font-medium text-gray-900 dark:text-gray-200"
    >
      {{ exporters }}
    </h2>
    <p class="leading-relaxed">Exporters</p>
  </div>
</template>

<script lang="ts" setup>
import { computed } from "vue";
import { useFetch } from "@vueuse/core";

const props = defineProps({
  refresh: {
    type: Number,
    default: 0,
  },
});

const url = computed(() => "/api/v0/console/widget/exporters?" + props.refresh);
const { data } = useFetch(url, { refetch: true })
  .get()
  .json<{ exporters: string[] } | { message: string }>();
const exporters = computed(() => {
  if (data.value && "exporters" in data.value) {
    return data.value.exporters.length;
  }
  return "???";
});
</script>
