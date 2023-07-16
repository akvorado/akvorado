<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <aside
    class="transition-height transition-width w-full shrink-0 duration-100 lg:h-auto"
    :class="open ? 'h-80 lg:w-72' : 'h-4 lg:w-4'"
  >
    <span
      class="absolute z-40 translate-x-4 transition-transform lg:translate-y-4"
      :class="
        open
          ? 'translate-y-80 rotate-180 lg:translate-x-72'
          : 'translate-y-4 lg:translate-x-0'
      "
    >
      <button
        class="flex h-4 w-4 cursor-pointer items-center justify-center rounded-full bg-white shadow transition-transform duration-100 hover:bg-gray-300 dark:bg-gray-900 dark:shadow-white/10 dark:hover:bg-black lg:translate-x-1/2 lg:translate-y-0"
        :class="open ? 'translate-y-1/2' : '-translate-y-1/2'"
        @click="open = !open"
      >
        <ChevronRightIcon class="hidden lg:inline" />
        <ChevronDownIcon class="lg:hidden" />
      </button>
    </span>
    <form
      class="h-full overflow-y-auto border-b border-gray-300 bg-gray-100 dark:border-slate-700 dark:bg-slate-800 lg:border-b-0 lg:border-r"
      autocomplete="off"
      spellcheck="false"
      @submit.prevent="submitOptions()"
    >
      <div v-if="open" class="flex flex-col px-3 py-4 lg:max-h-screen">
        <div
          class="mb-2 flex flex-row flex-wrap items-center justify-between gap-2 sm:max-lg:flex-nowrap"
        >
          <InputButton
            attr-type="submit"
            :disabled="hasErrors && !loading"
            :loading="loading"
            :type="loading ? 'alternative' : 'primary'"
            class="order-2 w-28 justify-center sm:max-lg:order-4"
            >{{ loading ? "Cancel" : applyLabel }}</InputButton
          >
          <InputChoice
            v-model="units"
            :choices="[
              { label: 'L3ᵇ⁄ₛ', name: 'l3bps' },
              { label: 'L2ᵇ⁄ₛ', name: 'l2bps' },
              { label: '→%', name: 'inl2%' },
              { label: '%→', name: 'outl2%' },
              { label: 'ᵖ⁄ₛ', name: 'pps' },
            ]"
            label="Unit"
            class="order-1"
          />
          <InputListBox
            v-model="graphType"
            :items="graphTypeList"
            class="order-3 grow basis-full sm:max-lg:order-3 sm:max-lg:basis-0"
            label="Graph type"
          >
            <template #selected>{{ graphType.name }}</template>
            <template #item="{ name }">
              <div class="flex w-full items-center justify-between">
                <span>{{ name }}</span>
                <GraphIcon
                  :name="name"
                  class="mr-1 inline h-4 text-gray-500 dark:text-gray-400"
                />
              </div>
            </template>
          </InputListBox>
          <div
            class="order-4 flex grow flex-row justify-between gap-x-3 sm:max-lg:order-2 sm:max-lg:grow-0 sm:max-lg:flex-col"
          >
            <InputCheckbox
              v-if="
                graphType.type === 'stacked' ||
                graphType.type === 'stacked100' ||
                graphType.type === 'lines' ||
                graphType.type === 'grid'
              "
              v-model="bidirectional"
              label="Bidirectional"
            />
            <InputCheckbox
              v-if="graphType.type === 'stacked'"
              v-model="previousPeriod"
              label="Previous period"
            />
          </div>
        </div>
        <SectionLabel>Time range</SectionLabel>
        <InputTimeRange v-model="timeRange" />
        <SectionLabel>Dimensions</SectionLabel>
        <InputDimensions
          v-model="dimensions"
          :min-dimensions="graphType.name === graphTypes.sankey ? 2 : 0"
        />
        <SectionLabel>
          <template #default>Filter</template>
          <template #hint>
            <kbd
              class="rounded border border-gray-300 bg-gray-200 px-1 dark:border-gray-600 dark:bg-gray-900"
              >Ctrl-Enter</kbd
            >
            to execute
          </template>
        </SectionLabel>
        <InputFilter v-model="filter" class="mb-2" @submit="submitOptions()" />
      </div>
    </form>
  </aside>
</template>

<script lang="ts" setup>
import { ref, watch, computed, inject, toRaw } from "vue";
import { Date as SugarDate } from "sugar-date";
import { ChevronDownIcon, ChevronRightIcon } from "@heroicons/vue/solid";
import {
  default as InputTimeRange,
  type ModelType as InputTimeRangeModelType,
} from "@/components/InputTimeRange.vue";
import {
  default as InputDimensions,
  type ModelType as InputDimensionsModelType,
} from "@/components/InputDimensions.vue";
import InputListBox from "@/components/InputListBox.vue";
import InputButton from "@/components/InputButton.vue";
import InputCheckbox from "@/components/InputCheckbox.vue";
import InputChoice from "@/components/InputChoice.vue";
import {
  default as InputFilter,
  type ModelType as InputFilterModelType,
} from "@/components/InputFilter.vue";
import { ServerConfigKey } from "@/components/ServerConfigProvider.vue";
import SectionLabel from "./SectionLabel.vue";
import GraphIcon from "./GraphIcon.vue";
import type { Units } from ".";
import { isEqual, omit } from "lodash-es";

const props = withDefaults(
  defineProps<{
    modelValue: ModelType;
    loading?: boolean;
  }>(),
  {
    loading: false,
  },
);
const emit = defineEmits<{
  "update:modelValue": [value: typeof props.modelValue];
  cancel: [];
}>();

const graphTypeList = Object.entries(graphTypes).map(([k, v], idx) => ({
  id: idx + 1,
  type: k as keyof typeof graphTypes, // why isn't it infered?
  name: v,
}));

const open = ref(false);
const graphType = ref(graphTypeList[0]);
const timeRange = ref<InputTimeRangeModelType>(null);
const dimensions = ref<InputDimensionsModelType>(null);
const filter = ref<InputFilterModelType>(null);
const units = ref<Units>("l3bps");
const bidirectional = ref(false);
const previousPeriod = ref(false);

const submitOptions = (force?: boolean) => {
  if (!force && props.loading) {
    emit("cancel");
  } else {
    if (options.value !== null && !hasErrors.value) {
      emit("update:modelValue", {
        ...options.value,
        start: SugarDate.create(options.value.humanStart).toISOString(),
        end: SugarDate.create(options.value.humanEnd).toISOString(),
      });
    }
  }
};

const options = computed((): InternalModelType => {
  if (!timeRange.value || !dimensions.value || !filter.value) {
    return options.value;
  }
  return {
    graphType: graphType.value.type,
    humanStart: timeRange.value?.start,
    humanEnd: timeRange.value?.end,
    dimensions: dimensions.value?.selected,
    limit: dimensions.value?.limit,
    "truncate-v4": dimensions.value?.truncate4,
    "truncate-v6": dimensions.value?.truncate6,
    filter: filter.value?.expression,
    units: units.value,
    bidirectional: false,
    previousPeriod: false,
    // Depending on the graph type...
    ...(graphType.value.type === "stacked" && {
      bidirectional: bidirectional.value,
      previousPeriod: previousPeriod.value,
    }),
    ...(graphType.value.type === "stacked100" && {
      bidirectional: bidirectional.value,
    }),
    ...(graphType.value.type === "lines" && {
      bidirectional: bidirectional.value,
    }),
    ...(graphType.value.type === "grid" && {
      bidirectional: bidirectional.value,
    }),
  };
});
const applyLabel = computed(() =>
  isEqual(options.value, omit(props.modelValue, ["start", "end"]))
    ? "Refresh"
    : "Apply",
);
const hasErrors = computed(
  () =>
    !!(
      timeRange.value?.errors ||
      dimensions.value?.errors ||
      filter.value?.errors
    ),
);

const serverConfiguration = inject(ServerConfigKey)!;
watch(
  () =>
    [
      props.modelValue,
      serverConfiguration.value?.defaultVisualizeOptions,
    ] as const,
  ([modelValue, defaultOptions]) => {
    if (!defaultOptions) return;
    const currentValue: NonNullable<InternalModelType> = modelValue ?? {
      graphType: defaultOptions.graphType,
      humanStart: defaultOptions.start,
      humanEnd: defaultOptions.end,
      dimensions: toRaw(defaultOptions.dimensions),
      limit: defaultOptions.limit,
      "truncate-v4": 32,
      "truncate-v6": 128,
      filter: defaultOptions.filter,
      units: "l3bps",
      bidirectional: false,
      previousPeriod: false,
    };

    // Dispatch values in refs
    const t = currentValue.graphType;
    graphType.value =
      graphTypeList.find(({ type }) => type === t) || graphTypeList[0];
    timeRange.value = {
      start: currentValue.humanStart,
      end: currentValue.humanEnd,
    };
    dimensions.value = {
      selected: [...currentValue.dimensions],
      limit: currentValue.limit,
      truncate4: currentValue["truncate-v4"] || 32,
      truncate6: currentValue["truncate-v6"] || 128,
    };
    filter.value = { expression: currentValue.filter };
    units.value = currentValue.units;
    bidirectional.value = currentValue.bidirectional;
    previousPeriod.value = currentValue.previousPeriod;

    // A bit risky, but it seems to work.
    if (
      modelValue === null ||
      !modelValue.start ||
      !modelValue.end ||
      !isEqual(omit(modelValue, ["start", "end"]), options.value)
    ) {
      open.value = true;
      submitOptions(true);
    }
  },
  { immediate: true, deep: true },
);
</script>

<script lang="ts">
import { graphTypes } from "./graphtypes";

export type ModelType = {
  graphType: keyof typeof graphTypes;
  start: string;
  end: string;
  humanStart: string;
  humanEnd: string;
  dimensions: string[];
  limit: number;
  "truncate-v4": number;
  "truncate-v6": number;
  filter: string;
  units: Units;
  bidirectional: boolean;
  previousPeriod: boolean;
} | null;
type InternalModelType = Omit<NonNullable<ModelType>, "start" | "end"> | null;
</script>
