<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div class="w-full md:flex md:flex-row">
    <div
      class="flex-none overflow-y-auto bg-gray-50 px-4 py-4 dark:bg-gray-900 dark:text-gray-200 print:hidden sm:px-6 md:mx-0 md:max-w-[30%] md:pl-0 md:pr-8"
    >
      <nav class="mx-auto text-sm md:mx-0">
        <ul class="space-y-1 pl-4">
          <li
            v-for="document in toc"
            :key="document.name"
            :class="{ 'mb-4': shouldShowTOCItem(document.name) }"
          >
            <div class="flex items-stretch items-center">
              <button
                type="button"
                class="mr-2 flex-none text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 flex items-center justify-center w-4 h-4"
                @click.stop="toggleTOCItem(document.name)"
              >
                <span
                  class="transform transition-transform duration-200"
                  :class="{ 'rotate-90': shouldShowTOCItem(document.name) }"
                  >❯</span
                >
              </button>
              <router-link
                :to="{
                  path: document.name,
                  hash: `#${document.headers[0].id}`,
                }"
                class="block font-semibold flex-1"
                :class="{
                  'text-blue-600': activeDocument === document.name,
                  'dark:text-blue-300': activeDocument === document.name,
                  'text-gray-900': activeDocument !== document.name,
                  'dark:text-gray-300': activeDocument !== document.name,
                }"
              >
                {{ document.headers[0].title }}
              </router-link>
            </div>
            <ul
              class="space-y-1 mt-2"
              :style="{
                height: shouldShowTOCItem(document.name) ? 'auto' : '0',
                overflow: shouldShowTOCItem(document.name)
                  ? 'visible'
                  : 'hidden',
                visibility: shouldShowTOCItem(document.name)
                  ? 'visible'
                  : 'hidden',
              }"
            >
              <template v-for="header in document.headers" :key="header.id">
                <li
                  v-if="header.level >= 2 && header.level <= 3"
                  :class="{
                    'ml-6': header.level == 2,
                    'ml-8': header.level == 3,
                  }"
                >
                  <router-link
                    :to="{ path: document.name, hash: `#${header.id}` }"
                    class="block"
                    :class="{
                      'text-blue-600':
                        activeDocument === document.name &&
                        activeSlug?.slice(1) === header.id,
                      'dark:text-blue-300':
                        activeDocument === document.name &&
                        activeSlug?.slice(1) === header.id,
                    }"
                  >
                    {{ header.title }}
                  </router-link>
                </li>
              </template>
            </ul>
          </li>
        </ul>
      </nav>
    </div>
    <div
      ref="contentEl"
      class="flex grow md:relative md:overflow-y-auto md:shadow-md md:dark:shadow-white/10"
    >
      <div class="max-w-full px-4 py-4 md:px-16">
        <InfoBox v-if="errorMessage" kind="error">
          <strong>Unable to fetch documentation page!</strong>
          {{ errorMessage }}
        </InfoBox>
        <!-- eslint-disable vue/no-v-html -->
        <div
          class="prose-img:center prose prose-sm mx-auto pb-2 dark:prose-invert md:prose-base prose-h1:border-b-2 prose-h1:border-gray-200 prose-pre:whitespace-pre-wrap prose-pre:break-all prose-pre:rounded dark:prose-h1:border-gray-700"
          v-html="markdown"
        ></div>
        <!-- eslint-enable -->
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { ref, computed, watch, inject, nextTick } from "vue";
import { useFetch } from "@vueuse/core";
import { useRouteHash } from "@vueuse/router";
import InfoBox from "@/components/InfoBox.vue";
import { TitleKey } from "@/components/TitleProvider.vue";

const props = defineProps<{ id: string }>();
const title = inject(TitleKey)!;

// Grab document
const url = computed(() => `/api/v0/console/docs/${props.id}`);
const { data, error } = useFetch(url, { refetch: true }).get().json<
  | { message: string } // on error
  | {
      markdown: string;
      toc: Array<{
        name: string;
        headers: Array<{ level: number; id: string; title: string }>;
      }>;
    }
>();
const errorMessage = computed(
  () =>
    (error.value &&
      data.value &&
      "message" in data.value &&
      (data.value.message || `Server returned an error: ${error.value}`)) ||
    "",
);
const markdown = computed(
  () =>
    (!error.value &&
      data.value &&
      "markdown" in data.value &&
      data.value.markdown) ||
    "",
);
const toc = computed(
  () =>
    (!error.value && data.value && "toc" in data.value && data.value.toc) || [],
);
const activeDocument = computed(() => props.id || null);
const activeSlug = useRouteHash();

// Expand TOC on user interaction or when switching document.
const expandedTOCItems = ref<Set<string>>(new Set());
watch(
  activeDocument,
  (newActiveDoc) => {
    if (newActiveDoc) {
      // Only keep the new active document expanded
      expandedTOCItems.value = new Set([newActiveDoc]);
    }
  },
  { immediate: true },
);
const toggleTOCItem = (documentName: string) => {
  if (expandedTOCItems.value.has(documentName)) {
    expandedTOCItems.value.delete(documentName);
  } else {
    expandedTOCItems.value.add(documentName);
  }
};
const shouldShowTOCItem = (documentName: string) => {
  return expandedTOCItems.value.has(documentName);
};

// Scroll to the right anchor after loading markdown
const contentEl = ref<HTMLElement | null>(null);
watch([markdown, activeSlug] as const, async () => {
  await nextTick();
  if (contentEl.value === null) return;
  let scrollEl = contentEl.value;
  while (
    window.getComputedStyle(scrollEl).position === "static" &&
    scrollEl.parentNode instanceof HTMLElement
  ) {
    scrollEl = scrollEl.parentNode;
  }
  const top =
    (activeSlug.value &&
      (
        document.querySelector(
          `#${CSS.escape(activeSlug.value.slice(1))}`,
        ) as HTMLElement | null
      )?.offsetTop) ||
    0;
  scrollEl.scrollTo(0, top);
});

// Update title
watch(markdown, async () => {
  await nextTick();
  const t = contentEl.value?.querySelector("h1")?.textContent;
  if (t) title.set(t);
});
</script>
