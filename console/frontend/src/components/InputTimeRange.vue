<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<!--
 Default, 2 columns:
 - Presets (2 columns)
 - Start, End
 Medium (md:), 3 columns:
 - Presets, Start, End
 Large (lg:), 1 column
-->

<template>
  <div class="grid grid-cols-2 gap-2 md:grid-cols-3 lg:grid-cols-1">
    <InputListBox
      v-model="selectedPreset"
      :items="presets"
      filter="name"
      label="Presets"
      class="col-span-full md:col-span-1"
    >
      <template #selected>{{ selectedPreset.name }}</template>
      <template #item="{ name }">{{ name }}</template>
    </InputListBox>
    <InputString v-model="startTime" label="Start" :error="startTimeError">
      <template #button>
        <button
          type="button"
          title="Move the time range backward"
          :disabled="hasErrors"
          :class="shiftButtonClass"
          @click="shiftTimeRange(-1)"
        >
          <ChevronLeftIcon class="h-5 w-5" aria-hidden="true" />
        </button>
      </template>
    </InputString>
    <InputString v-model="endTime" label="End" :error="endTimeError">
      <template #button>
        <button
          type="button"
          title="Move the time range forward"
          :disabled="hasErrors"
          :class="shiftButtonClass"
          @click="shiftTimeRange(1)"
        >
          <ChevronRightIcon class="h-5 w-5" aria-hidden="true" />
        </button>
      </template>
    </InputString>
  </div>
</template>

<script lang="ts" setup>
import { ref, computed, watch, nextTick } from "vue";
import { Date as SugarDate } from "sugar-date";
import { ChevronLeftIcon, ChevronRightIcon } from "@heroicons/vue/solid";
import InputString from "@/components/InputString.vue";
import InputListBox from "@/components/InputListBox.vue";
import { isEqual } from "lodash-es";

const props = defineProps<{
  modelValue: ModelType;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: typeof props.modelValue];
  submit: [];
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

const shiftButtonClass =
  "cursor-pointer text-gray-400 hover:text-gray-600 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:text-gray-400 dark:hover:text-gray-200 dark:disabled:hover:text-gray-400";

const formatDate = (date: Date) =>
  SugarDate(date).format("{yyyy}-{MM}-{dd} {HH}:{mm}:{ss}").raw;

// Move both ends of the time range by its own duration. A negative direction
// moves to the past, a positive one to the future.
const shiftTimeRange = (direction: number) => {
  if (hasErrors.value) return;
  const { start, end } = parsedTimes.value;
  const offset = direction * (end.valueOf() - start.valueOf());
  startTime.value = formatDate(new Date(start.valueOf() + offset));
  endTime.value = formatDate(new Date(end.valueOf() + offset));
  nextTick(() => emit("submit"));
};

// Double the duration of the time range, keeping the same center.
const zoomOutTimeRange = () => {
  if (hasErrors.value) return;
  const { start, end } = parsedTimes.value;
  const duration = end.valueOf() - start.valueOf();
  if (endTime.value.trim().toLowerCase() === "now") {
    startTime.value = formatDate(new Date(start.valueOf() - duration));
  } else {
    startTime.value = formatDate(new Date(start.valueOf() - duration / 2));
    endTime.value = formatDate(new Date(end.valueOf() + duration / 2));
  }
  nextTick(() => emit("submit"));
};

defineExpose({ zoomOut: zoomOutTimeRange });

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
