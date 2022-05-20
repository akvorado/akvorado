<template>
  <div class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-1">
    <InputListBox
      v-model="selectedPreset"
      :items="presets"
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

<script setup>
const props = defineProps({
  modelValue: {
    // start: start time
    // end: end time
    // errors: is there an input error?
    type: Object,
    required: true,
  },
});
const emit = defineEmits(["update:modelValue"]);

import { ref, computed, watch } from "vue";
import { Date as SugarDate } from "sugar-date";
import InputString from "@/components/InputString.vue";
import InputListBox from "@/components/InputListBox.vue";
import isEqual from "lodash.isequal";

const startTime = ref("");
const endTime = ref("");
const parsedStartTime = computed(() => SugarDate.create(startTime.value));
const parsedEndTime = computed(() => SugarDate.create(endTime.value));
const startTimeError = computed(() =>
  isNaN(parsedStartTime.value) ? "Invalid date" : ""
);
const endTimeError = computed(
  () =>
    (isNaN(parsedEndTime.value) ? "Invalid date" : "") ||
    (!isNaN(parsedStartTime.value) &&
      parsedStartTime.value > parsedEndTime.value &&
      "End date should be before start date") ||
    ""
);
const hasErrors = computed(
  () => !!(startTimeError.value || endTimeError.value)
);

const presets = [
  { name: "Custom" },
  { name: "Last hour", start: "1 hour ago", end: "now" },
  { name: "Last 6 hours", start: "6 hours ago", end: "now" },
  { name: "Last 12 hours", start: "12 hours ago", end: "now" },
  { name: "Last 24 hours", start: "24 hours ago", end: "now" },
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
    start: "1 week from today",
    end: "6 days from today",
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
    startTime.value = m.start;
    endTime.value = m.end;
  },
  { immediate: true, deep: true }
);
watch(
  [startTime, endTime, hasErrors],
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
  { immediate: true }
);
</script>
