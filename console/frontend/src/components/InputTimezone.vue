<!-- SPDX-FileCopyrightText: 2026 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div class="mt-2 text-xs">
    <!-- Collapsed summary bar & toggle button -->
    <div
      class="flex items-center justify-between rounded border border-gray-200 bg-gray-50 px-2 py-1.5 dark:border-gray-700 dark:bg-gray-800/80"
    >
      <div class="flex min-w-0 items-center gap-1.5 truncate">
        <GlobeIcon class="h-4 w-4 shrink-0 text-gray-400 dark:text-gray-400" />
        <span class="truncate font-medium text-gray-800 dark:text-gray-200">
          {{ currentItem.name }}
        </span>
        <span
          v-if="currentItem.abbr && currentItem.abbr !== currentItem.name"
          class="shrink-0 text-gray-400 dark:text-gray-400"
        >
          {{ currentItem.abbr }}
        </span>
        <span
          class="shrink-0 rounded bg-gray-200 px-1 py-0.5 font-mono text-[10px] text-gray-600 dark:bg-gray-700 dark:text-gray-300"
        >
          {{ currentItem.offsetStr }}
        </span>
      </div>
      <button
        type="button"
        class="ml-2 flex shrink-0 cursor-pointer items-center gap-1 rounded border border-gray-300 bg-white px-1.5 py-0.5 text-xs text-gray-700 hover:bg-gray-100 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600"
        @click="isOpen = !isOpen"
      >
        <span>Change time settings</span>
        <ChevronUpIcon v-if="isOpen" class="h-3.5 w-3.5" />
        <ChevronDownIcon v-else class="h-3.5 w-3.5" />
      </button>
    </div>

    <!-- Expanded panel -->
    <div
      v-if="isOpen"
      class="mt-1.5 rounded border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-900"
    >
      <!-- Tabs -->
      <div class="flex border-b border-gray-200 px-2 pt-1 dark:border-gray-700">
        <button
          type="button"
          class="border-b-2 border-orange-500 pb-1 text-xs font-semibold text-gray-800 dark:text-white"
        >
          Time zone
        </button>
      </div>

      <!-- Search input -->
      <div class="p-2 pb-1">
        <div class="relative">
          <input
            ref="searchInput"
            v-model="searchQuery"
            type="text"
            placeholder="Type to search (country, city, abbreviation)"
            class="w-full rounded border border-gray-300 bg-white py-1.5 pl-2.5 pr-7 text-xs text-gray-800 placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-400"
            @keydown.esc="isOpen = false"
          />
          <SearchIcon
            class="pointer-events-none absolute right-2 top-2 h-4 w-4 text-gray-400 dark:text-gray-400"
          />
        </div>
      </div>

      <!-- Timezone list -->
      <div class="max-h-60 overflow-y-auto px-1 py-1">
        <!-- Quick / Default options -->
        <div v-if="filteredData.quick.length > 0" class="mb-1">
          <button
            v-for="item in filteredData.quick"
            :key="item.id"
            type="button"
            class="flex w-full cursor-pointer items-center justify-between rounded px-2 py-1.5 text-left text-xs transition-colors hover:bg-gray-100 dark:hover:bg-gray-800"
            :class="{
              'bg-blue-50 text-blue-700 font-medium dark:bg-blue-900/40 dark:text-blue-300':
                isSelected(item),
              'text-gray-700 dark:text-gray-200': !isSelected(item),
            }"
            @click="selectTimezone(item.id)"
          >
            <div class="flex items-center gap-1.5 truncate">
              <CheckIcon
                class="h-3.5 w-3.5 shrink-0 text-blue-600 dark:text-blue-400"
                :class="{ invisible: !isSelected(item) }"
              />
              <span class="truncate font-medium">{{ item.name }}</span>
              <span class="truncate text-gray-400 dark:text-gray-400">{{
                item.abbr
              }}</span>
            </div>
            <span
              class="shrink-0 rounded bg-gray-100 px-1 py-0.5 font-mono text-[10px] text-gray-600 dark:bg-gray-800 dark:text-gray-300"
            >
              {{ item.offsetStr }}
            </span>
          </button>
        </div>

        <div
          v-if="filteredData.quick.length > 0 && filteredData.groups.length > 0"
          class="my-1 border-t border-gray-100 dark:border-gray-800"
        ></div>

        <!-- Grouped options -->
        <div
          v-for="group in filteredData.groups"
          :key="group.region"
          class="mb-2"
        >
          <div
            class="sticky top-0 z-10 bg-white/95 px-2 py-0.5 text-[11px] font-semibold text-gray-400 uppercase tracking-wider backdrop-blur-xs dark:bg-gray-900/95 dark:text-gray-400"
          >
            {{ group.region }}
          </div>
          <button
            v-for="item in group.items"
            :key="item.id"
            type="button"
            class="flex w-full cursor-pointer items-center justify-between rounded px-2 py-1.5 text-left text-xs transition-colors hover:bg-gray-100 dark:hover:bg-gray-800"
            :class="{
              'bg-blue-50 text-blue-700 font-medium dark:bg-blue-900/40 dark:text-blue-300':
                isSelected(item),
              'text-gray-700 dark:text-gray-200': !isSelected(item),
            }"
            @click="selectTimezone(item.id)"
          >
            <div class="flex items-center gap-1.5 truncate">
              <CheckIcon
                class="h-3.5 w-3.5 shrink-0 text-blue-600 dark:text-blue-400"
                :class="{ invisible: !isSelected(item) }"
              />
              <span class="truncate">{{ item.name }}</span>
              <span
                v-if="item.abbr && item.abbr !== item.name"
                class="truncate text-gray-400 dark:text-gray-400"
                >{{ item.abbr }}</span
              >
            </div>
            <span
              class="shrink-0 rounded bg-gray-100 px-1 py-0.5 font-mono text-[10px] text-gray-600 dark:bg-gray-800 dark:text-gray-300"
            >
              {{ item.offsetStr }}
            </span>
          </button>
        </div>

        <div
          v-if="
            filteredData.quick.length === 0 && filteredData.groups.length === 0
          "
          class="py-4 text-center text-gray-400 dark:text-gray-400"
        >
          No timezones matching "{{ searchQuery }}"
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { ref, computed, nextTick, watch } from "vue";
import {
  GlobeIcon,
  SearchIcon,
  CheckIcon,
  ChevronDownIcon,
  ChevronUpIcon,
} from "@heroicons/vue/solid";
import { useTimezone } from "./TimezoneProvider.vue";
import {
  searchTimezones,
  getQuickTimezones,
  getAllTimezoneItems,
  type TimezoneItem,
} from "@/utils/timezone";

const {
  timezone,
  setTimezone,
  timezoneOffsetLabel,
  timezoneAbbr,
  resolvedTimezone,
} = useTimezone();

const isOpen = ref(false);
const searchQuery = ref("");
const searchInput = ref<HTMLInputElement>();

watch(isOpen, (open) => {
  if (open) {
    searchQuery.value = "";
    nextTick(() => {
      searchInput.value?.focus();
    });
  }
});

const filteredData = computed(() => searchTimezones(searchQuery.value));

const currentItem = computed((): TimezoneItem => {
  const quick = getQuickTimezones();
  const matchQuick = quick.find((it) => it.id === timezone.value);
  if (matchQuick) return matchQuick;

  const all = getAllTimezoneItems();
  const matchAll = all.find((it) => it.id === timezone.value);
  if (matchAll) return matchAll;

  return {
    id: timezone.value,
    name: timezone.value,
    fullName: timezone.value,
    region: "",
    offsetStr: timezoneOffsetLabel.value,
    offsetMinutes: 0,
    abbr: timezoneAbbr.value,
    searchKey: "",
  };
});

const isSelected = (item: TimezoneItem) => {
  if (item.id === "browser" && timezone.value === "browser") return true;
  if (
    item.id === "UTC" &&
    (timezone.value === "UTC" || timezone.value === "Etc/UTC")
  )
    return true;
  return item.id === timezone.value || item.id === resolvedTimezone.value;
};

const selectTimezone = (id: string) => {
  setTimezone(id);
  isOpen.value = false;
};
</script>
