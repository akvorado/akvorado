<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <div class="w-full md:flex md:flex-row">
    <div
      class="max-w-[30%] flex-none overflow-y-auto bg-gray-50 py-4 px-4 dark:bg-gray-900 dark:text-gray-200 print:hidden sm:px-6 md:mx-0 md:pl-0 md:pr-8"
    >
      <nav class="mx-auto text-sm md:mx-0">
        <ul class="space-y-1 pl-6 md:space-y-8">
          <li v-for="document in toc" :key="document.name" class="md:space-y-2">
            <router-link
              :to="{ path: document.name, hash: `#${document.headers[0].id}` }"
              class="block font-semibold"
              :class="{
                'text-blue-600': activeDocument === document.name,
                'dark:text-blue-300': activeDocument === document.name,
                'text-gray-900': activeDocument !== document.name,
                'dark:text-gray-300': activeDocument !== document.name,
              }"
            >
              {{ document.headers[0].title }}
            </router-link>
            <ul
              class="space-y-1 md:block md:space-y-2"
              :class="{ hidden: activeDocument !== document.name }"
            >
              <template v-for="header in document.headers" :key="header.id">
                <li
                  v-if="header.level >= 2 && header.level <= 3"
                  :class="{
                    'ml-2': header.level == 2,
                    'ml-4': header.level == 3,
                  }"
                >
                  <router-link
                    :to="{ path: document.name, hash: `#${header.id}` }"
                    class="block"
                    :class="{
                      'text-blue-600':
                        activeDocument === document.name &&
                        activeSlug.slice(1) === header.id,
                      'dark:text-blue-300':
                        activeDocument === document.name &&
                        activeSlug.slice(1) === header.id,
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
      <div class="max-w-full py-4 px-4 md:px-16">
        <InfoBox v-if="errorMessage" kind="danger">
          <strong>Unable to fetch documentation page!</strong>
          {{ errorMessage }}
        </InfoBox>
        <div
          class="prose-img:center prose prose-sm mx-auto prose-h1:border-b-2 prose-pre:rounded dark:prose-invert dark:prose-h1:border-gray-700 md:prose-base"
          v-html="markdown"
        ></div>
      </div>
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  id: {
    type: String,
    required: true,
  },
});

import { ref, computed, watch, inject, nextTick } from "vue";
import { useFetch } from "@vueuse/core";
import { useRouteHash } from "@vueuse/router";
import InfoBox from "@/components/InfoBox.vue";

const title = inject("title");

// Grab document
const url = computed(() => `/api/v0/console/docs/${props.id}`);
const { data, error } = useFetch(url, { refetch: true }).get().json();
const errorMessage = computed(
  () =>
    (error.value &&
      (data.value?.message || `Server returned an error: ${error.value}`)) ||
    ""
);
const markdown = computed(() => (!error.value && data.value?.markdown) || "");
const toc = computed(() => (!error.value && data.value?.toc) || []);
const activeDocument = computed(() => props.id || null);
const activeSlug = useRouteHash();

// Scroll to the right anchor after loading markdown
const contentEl = ref(null);
watch([markdown, activeSlug], async () => {
  await nextTick();
  let scrollEl = contentEl.value;
  while (window.getComputedStyle(scrollEl).position === "static") {
    scrollEl = scrollEl.parentNode;
  }
  const top =
    (activeSlug.value &&
      document.querySelector(`#${CSS.escape(activeSlug.value.slice(1))}`)
        ?.offsetTop) ||
    0;
  scrollEl.scrollTo(0, top);
});

// Update title
watch(markdown, async () => {
  await nextTick();
  title.set(contentEl.value?.querySelector("h1")?.textContent);
});
</script>
