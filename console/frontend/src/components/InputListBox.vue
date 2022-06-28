<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <component
    :is="component.Box"
    :class="$attrs['class']"
    :multiple="multiple"
    :model-value="modelValue"
    @update:model-value="(item) => $emit('update:modelValue', item)"
  >
    <div class="relative">
      <InputBase v-slot="{ id, childClass }" v-bind="otherAttrs" :error="error">
        <component :is="component.Widget" :id="id" :class="childClass">
          <div class="flex flex-wrap items-center gap-x-2 pr-6 text-left">
            <slot name="selected"></slot>
            <component
              :is="component.Input"
              class="w-10 grow border-none bg-transparent p-0 focus:outline-none"
              placeholder="Search..."
              @change="query = $event.target.value"
              @focus="query = ''"
            >
            </component>
          </div>
          <component
            :is="component.Button"
            :id="id"
            class="absolute inset-y-0 right-0 flex items-center pr-2"
            :class="{ 'pointer-events-none': !component.Input }"
          >
            <SelectorIcon class="h-5 w-5 text-gray-400" aria-hidden="true" />
          </component>
        </component>
      </InputBase>

      <transition
        enter-active-class="transition duration-100 ease-out"
        enter-from-class="transform scale-95 opacity-0"
        enter-to-class="transform scale-100 opacity-100"
        leave-active-class="transition duration-75 ease-out"
        leave-from-class="transform scale-100 opacity-100"
        leave-to-class="transform scale-95 opacity-0"
      >
        <component
          :is="component.Options"
          class="absolute z-50 max-h-60 w-full overflow-auto rounded bg-white py-1 text-sm text-gray-700 shadow dark:bg-gray-900 dark:text-gray-200 dark:shadow-white/10"
        >
          <div
            v-if="filteredItems.length === 0 && query != ''"
            class="py-2 px-2.5"
          >
            <slot name="nomatch" :query="query">
              <div
                class="cursor-not-allowed select-none text-gray-700 dark:text-gray-300"
              >
                No results
              </div>
            </slot>
          </div>
          <component
            :is="component.Option"
            v-for="item in filteredItems"
            v-slot="{ selected, active }"
            :key="item.id"
            :value="item"
            as="template"
          >
            <li
              class="relative inline-flex w-full cursor-default select-none py-2 pl-10 pr-2.5 text-sm hover:bg-gray-100 dark:hover:bg-gray-600 dark:hover:text-white"
              :class="{
                'bg-gray-100 dark:bg-gray-600 dark:bg-gray-600 dark:text-white':
                  active,
              }"
            >
              <span class="block w-full truncate">
                <slot name="item" v-bind="item"></slot>
              </span>
              <span
                v-if="selected"
                class="absolute inset-y-0 left-0 flex items-center pl-2.5 text-blue-600 dark:text-blue-500"
              >
                <CheckIcon class="h-5 w-5" aria-hidden="true" />
              </span>
            </li>
          </component>
        </component>
      </transition>
    </div>
  </component>
</template>

<script>
export default {
  inheritAttrs: false,
};
</script>

<script setup>
const props = defineProps({
  modelValue: {
    type: Object,
    required: true,
  },
  error: {
    type: String,
    default: "",
  },
  items: {
    // Each item in the array is expected to have "id".
    type: Array,
    required: true,
  },
  multiple: {
    // Allow to select multiple values.
    type: Boolean,
    default: false,
  },
  filter: {
    // Enable filtering on the provided property.
    type: String,
    default: null,
  },
});
defineEmits(["update:modelValue"]);

import { ref, computed, useAttrs } from "vue";
import {
  Listbox,
  Combobox,
  ListboxButton,
  ComboboxButton,
  ListboxOptions,
  ComboboxOptions,
  ListboxOption,
  ComboboxOption,
  ComboboxInput,
} from "@headlessui/vue";
import { CheckIcon, SelectorIcon } from "@heroicons/vue/solid";
import InputBase from "@/components/InputBase.vue";

const attrs = useAttrs();
const query = ref("");
const component = computed(() =>
  props.filter === null
    ? {
        Box: Listbox,
        Widget: ListboxButton,
        Button: "span",
        Options: ListboxOptions,
        Option: ListboxOption,
      }
    : {
        Box: Combobox,
        Widget: "div",
        Button: ComboboxButton,
        Options: ComboboxOptions,
        Option: ComboboxOption,
        Input: ComboboxInput,
      }
);
const filteredItems = computed(() =>
  props.filter === null
    ? props.items
    : props.items.filter((it) =>
        query.value
          .toLowerCase()
          .split(/\W+/)
          .every((w) => it[props.filter].toLowerCase().includes(w))
      )
);
const otherAttrs = computed(() => {
  // eslint-disable-next-line no-unused-vars
  const { class: _, ...others } = attrs;
  return others;
});
</script>
