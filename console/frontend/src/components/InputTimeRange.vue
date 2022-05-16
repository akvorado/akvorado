<template>
  <div class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-1">
    <Listbox v-model="selectedPreset" class="col-span-2 sm:col-span-1">
      <div class="relative">
        <ListboxButton
          id="preset"
          class="peer block w-full appearance-none rounded-t-lg border-0 border-b-2 border-gray-300 bg-gray-50 px-2.5 pb-1.5 pt-4 text-left text-sm text-gray-900 focus:border-blue-600 focus:outline-none focus:ring-0 dark:border-gray-600 dark:bg-gray-700 dark:text-white dark:focus:border-blue-500"
        >
          <span class="block truncate">{{ selectedPreset.name }}</span>
          <span
            class="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2"
          >
            <SelectorIcon class="h-5 w-5 text-gray-400" aria-hidden="true" />
          </span>
        </ListboxButton>
        <label
          for="preset"
          class="z-5 absolute top-3 left-2.5 origin-[0] -translate-y-3 scale-75 transform text-sm text-gray-500 peer-focus:text-blue-600 dark:text-gray-400 dark:peer-focus:text-blue-500"
        >
          Preset
        </label>

        <transition
          leave-active-class="transition duration-100 ease-in"
          leave-from-class="opacity-100"
          leave-to-class="opacity-0"
          class="z-10 rounded bg-white shadow dark:bg-gray-700"
        >
          <ListboxOptions
            class="absolute max-h-60 w-full overflow-auto py-1 text-sm text-gray-700 dark:text-gray-200"
          >
            <ListboxOption
              v-for="preset in presets"
              v-slot="{ active, selected }"
              :key="preset.id"
              :value="preset"
              as="template"
            >
              <li
                class="relative inline-flex w-full cursor-default select-none py-2 pl-10 pr-4 text-sm hover:bg-gray-100 dark:hover:bg-gray-600 dark:hover:text-white"
                :class="
                  active &&
                  'bg-gray-100 dark:bg-gray-600 dark:bg-gray-600 dark:text-white'
                "
              >
                <span class="block truncate">{{ preset.name }}</span>
                <span
                  v-if="selected"
                  class="absolute inset-y-0 left-0 flex items-center pl-3 text-blue-600 dark:text-blue-500"
                >
                  <CheckIcon class="h-5 w-5" aria-hidden="true" />
                </span>
              </li>
            </ListboxOption>
          </ListboxOptions>
        </transition>
      </div>
    </Listbox>
    <InputString
      id="start"
      v-model="startTime"
      label="Start"
      :error="startTimeError"
    />
    <InputString id="end" v-model="endTime" label="End" :error="endTimeError" />
  </div>
</template>

<script setup>
import { ref, computed, watch } from "vue";
import { Date as SugarDate } from "sugar-date";
import {
  Listbox,
  ListboxButton,
  ListboxOptions,
  ListboxOption,
} from "@headlessui/vue";
import { CheckIcon, SelectorIcon } from "@heroicons/vue/solid";
import InputString from "./InputString.vue";

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
watch(
  selectedPreset,
  (preset) => {
    if (preset.start) {
      startTime.value = preset.start;
      endTime.value = preset.end;
    }
  },
  { deep: true }
);

watch(
  () => props.modelValue,
  (m) => {
    startTime.value = m.start;
    endTime.value = m.end;
  },
  { immediate: true, deep: true }
);
watch([startTime, endTime, hasErrors], ([start, end, errors]) => {
  if (selectedPreset.value.start != start || selectedPreset.value.end != end) {
    selectedPreset.value = presets[0];
  }
  emit("update:modelValue", {
    start,
    end,
    errors,
  });
});
</script>
