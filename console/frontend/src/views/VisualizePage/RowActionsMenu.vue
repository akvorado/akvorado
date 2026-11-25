<!-- SPDX-FileCopyrightText: 2026 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <Teleport to="body">
    <div
      v-if="selection"
      ref="menu"
      role="menu"
      class="fixed z-50 w-64 rounded bg-white text-sm text-gray-700 shadow-lg ring-1 ring-black/10 dark:bg-gray-700 dark:text-gray-200 dark:ring-white/10"
      :style="position"
      @keydown.escape="emit('close')"
    >
      <div
        class="truncate border-b border-gray-200 px-3 py-2 text-xs text-gray-500 dark:border-gray-600 dark:text-gray-400"
        :title="label"
      >
        {{ label }}
      </div>
      <div v-if="!showingDimensions" class="py-1">
        <button
          type="button"
          role="menuitem"
          :class="itemClass"
          @click="emit('include')"
        >
          <FilterIcon class="h-4 w-4" aria-hidden="true" />
          Keep only this
        </button>
        <button
          type="button"
          role="menuitem"
          :class="itemClass"
          @click="emit('exclude')"
        >
          <BanIcon class="h-4 w-4" aria-hidden="true" />
          Exclude this
        </button>
        <button
          type="button"
          role="menuitem"
          :class="itemClass"
          @click="showingDimensions = true"
        >
          <SwitchVerticalIcon class="h-4 w-4" aria-hidden="true" />
          <span class="grow">Show by…</span>
          <ChevronRightIcon class="h-4 w-4" aria-hidden="true" />
        </button>
      </div>
      <div v-else class="py-1">
        <div class="px-2 pb-1">
          <input
            ref="searchInput"
            v-model="search"
            type="search"
            placeholder="Search dimensions"
            aria-label="Search dimensions"
            class="w-full rounded border border-gray-300 bg-white px-2 py-1 text-sm dark:border-gray-600 dark:bg-gray-800"
            @keydown.enter.prevent="
              matchingDimensions[0] && emit('showBy', matchingDimensions[0])
            "
          />
        </div>
        <ul class="max-h-60 overflow-y-auto">
          <li v-for="dimension in matchingDimensions" :key="dimension">
            <button
              type="button"
              role="menuitem"
              :class="itemClass"
              @click="emit('showBy', dimension)"
            >
              {{ dimension }}
            </button>
          </li>
        </ul>
      </div>
    </div>
  </Teleport>
</template>

<script lang="ts" setup>
import { computed, inject, nextTick, ref, watch } from "vue";
import { onClickOutside, useEventListener } from "@vueuse/core";
import {
  BanIcon,
  ChevronRightIcon,
  FilterIcon,
  SwitchVerticalIcon,
} from "@heroicons/vue/solid";
import { ServerConfigKey } from "@/components/ServerConfigProvider.vue";

const props = defineProps<{
  selection: RowSelection | null;
}>();
const emit = defineEmits<{
  close: [];
  include: [];
  exclude: [];
  showBy: [dimension: string];
}>();

const serverConfiguration = inject(ServerConfigKey)!;
const menu = ref<HTMLElement | null>(null);
const searchInput = ref<HTMLInputElement | null>(null);
const showingDimensions = ref(false);
const search = ref("");

const itemClass =
  "flex w-full cursor-pointer items-center gap-2 px-3 py-1.5 text-left hover:bg-gray-100 focus:bg-gray-100 focus:outline-none dark:hover:bg-gray-600 dark:focus:bg-gray-600";

const label = computed(() =>
  props.selection?.values.join(" — ").replace(/^$/, "Total"),
);
const matchingDimensions = computed(() => {
  const term = search.value.trim().toLowerCase();
  return (serverConfiguration.value?.dimensions ?? []).filter(
    (dimension) =>
      !props.selection?.dimensions.includes(dimension) &&
      dimension.toLowerCase().includes(term),
  );
});

// Open the menu on the side of the pointer with the most room.
const position = computed(() => {
  if (!props.selection) return {};
  const { x, y } = props.selection;
  return {
    ...(x > window.innerWidth / 2
      ? { right: `${window.innerWidth - x}px` }
      : { left: `${x}px` }),
    ...(y > window.innerHeight / 2
      ? { bottom: `${window.innerHeight - y}px` }
      : { top: `${y}px` }),
  };
});

watch(
  () => props.selection,
  async (selection) => {
    showingDimensions.value = false;
    search.value = "";
    if (!selection) return;
    await nextTick();
    menu.value?.querySelector<HTMLElement>("[role=menuitem]")?.focus();
  },
);
watch(showingDimensions, async (showing) => {
  if (!showing) return;
  await nextTick();
  searchInput.value?.focus();
});

const close = () => {
  if (props.selection) emit("close");
};
onClickOutside(menu, close);
useEventListener(window, "resize", close);
useEventListener(
  window,
  "scroll",
  (event: Event) => {
    if (!menu.value?.contains(event.target as Node)) close();
  },
  { capture: true, passive: true },
);
</script>

<script lang="ts">
// A row of results selected by the user, with the position of the pointer.
export type RowSelection = {
  dimensions: string[];
  values: string[];
  x: number;
  y: number;
};
</script>
