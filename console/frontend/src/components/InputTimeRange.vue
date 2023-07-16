<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-1">
    <InputListBox
      v-model="selectedPreset"
      :items="presets"
      filter="name"
      label="Presets"
      class="col-span-2 sm:col-span-1"
    >
      <template #selected>{{ selectedPreset.name }}</template>
      <template #item="{ name }">{{ name }}</template>
    </InputListBox>
    <InputString v-model="startTime" label="Start" :error="startTimeError" />
    <InputString v-model="endTime" label="End" :error="endTimeError" />
  </div>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from "vue";
import { Date as SugarDate } from "sugar-date";
import InputString from "@/components/InputString.vue";
import InputListBox from "@/components/InputListBox.vue";
import { isEqual } from "lodash-es";

const props = defineProps<{
  modelValue: ModelType;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: typeof props.modelValue];
}>();

const startTime = ref("");
const endTime = ref("");
const parsedTimes = computed(() => ({
  start: SugarDate.create(startTime.value),
  end: SugarDate.create(endTime.value),
}));
const startTimeError = computed(() =>
  isNaN(parsedTimes.value.start.valueOf()) ? "Invalid date" : "",
);
const endTimeError = computed(
  () =>
    (isNaN(parsedTimes.value.end.valueOf()) ? "Invalid date" : "") ||
    (!isNaN(parsedTimes.value.start.valueOf()) &&
      parsedTimes.value.start > parsedTimes.value.end &&
      "End date should be before start date") ||
    "",
);
const hasErrors = computed(
  () => !!(startTimeError.value || endTimeError.value),
);

const presets = [
  { name: "Custom" },
  { name: "Last hour", start: "1 hour ago", end: "now" },
  { name: "Last 6 hours", start: "6 hours ago", end: "now" },
  { name: "Last 12 hours", start: "12 hours ago", end: "now" },
  { name: "Last 24 hours", start: "24 hours ago", end: "now" },
  { name: "Last evening", start: "yesterday at 7pm", end: "today at 1am" },
  { name: "Last 2 days", start: "2 days ago", end: "now" },
  { name: "Last 7 days", start: "7 days ago", end: "now" },
  { name: "Last 30 days", start: "30 days ago", end: "now" },
  { name: "Last 3 months", start: "3 months ago", end: "now" },
  { name: "Last 6 months", start: "6 months ago", end: "now" },
  { name: "Last year", start: "1 year ago", end: "now" },
  { name: "Last 2 years", start: "2 years ago", end: "now" },
  { name: "Last 5 years", start: "5 years ago", end: "now" },
  { name: "Today", start: "today", end: "end of today" },
  { name: "Yesterday", start: "yesterday", end: "end of yesterday" },
  {
    name: "Day before yesterday",
    start: "day before yesterday",
    end: "yesterday",
  },
  {
    name: "This week",
    start: "the beginning of this week",
    end: "the end of this week",
  },
  {
    name: "This month",
    start: "the beginning of this month",
    end: "the end of this month",
  },
  {
    name: "This year",
    start: "the beginning of this year",
    end: "the end of this year",
  },
  {
    name: "This day last week",
    start: "0am 1 week ago",
    end: "0am 6 days ago",
  },
  {
    name: "Previous week",
    start: "the beginning of last week",
    end: "the end of last week",
  },
  {
    name: "Previous month",
    start: "the beginning of last month",
    end: "the end of last month",
  },
  {
    name: "Previous year",
    start: "the beginning of last year",
    end: "the end of last year",
  },
].map((v, idx) => ({ id: idx + 1, ...v }));
const selectedPreset = ref(presets[0]);
watch(selectedPreset, (preset) => {
  if (preset.start) {
    startTime.value = preset.start;
    endTime.value = preset.end;
  }
});

watch(
  () => props.modelValue,
  (m) => {
    if (m) {
      startTime.value = m.start;
      endTime.value = m.end;
    }
  },
  { immediate: true, deep: true },
);
watch(
  [startTime, endTime, hasErrors] as const,
  ([start, end, errors]) => {
    // Find the right preset
    const newPreset =
      presets.find((p) => p.start === start && p.end === end) || presets[0];
    if (newPreset.id !== selectedPreset.value.id) {
      selectedPreset.value = newPreset;
    }

    // Update the model
    const newModel = {
      start,
      end,
      errors,
    };
    if (!isEqual(newModel, props.modelValue)) {
      emit("update:modelValue", newModel);
    }
  },
  { immediate: true },
);
</script>

<script lang="ts">
export type ModelType = {
  start: string;
  end: string;
  errors?: boolean;
} | null;
</script>
