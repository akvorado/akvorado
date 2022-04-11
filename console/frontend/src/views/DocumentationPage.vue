<template>
  <div class="w-full md:flex md:flex-row">
    <div
      class="flex-none overflow-y-auto bg-gray-50 py-4 px-4 dark:bg-gray-900 dark:text-gray-200 sm:px-6 md:mx-0 md:pl-0 md:pr-8"
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
                        activeSlug === header.id,
                      'dark:text-blue-300':
                        activeDocument === document.name &&
                        activeSlug === header.id,
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
      class="flex grow md:relative md:overflow-y-auto md:shadow-md"
    >
      <div class="max-w-full py-4 px-4 md:px-16">
        <div
          class="prose-img:center prose prose-sm mx-auto prose-h1:border-b-2 prose-pre:rounded dark:prose-invert dark:prose-h1:border-gray-700 md:prose-base"
          v-html="markdown"
        ></div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick } from "vue";
import { useRoute } from "vue-router";
import { notify } from "notiwind";

const route = useRoute();
const markdown = ref("");
const toc = ref([]);
const contentEl = ref(null);
const activeDocument = ref(null);
const activeSlug = computed(() => route.hash.replace(/^#/, ""));

watch(
  () => ({ id: route.params.id, hash: route.hash }),
  async (to, from) => {
    if (to.id === undefined)
      return;
    if (to.id !== from?.id) {
      const id = to.id;
      try {
        const response = await fetch(`/api/v0/docs/${id}`);
        if (!response.ok) {
          throw `got a ${response.status} error`;
        }
        const data = await response.json();
        markdown.value = data.markdown;
        toc.value = data.toc;
        activeDocument.value = id;
      } catch (error) {
        console.error(`while retrieving ${id}:`, error);
        notify(
          {
            group: "top",
            kind: "error",
            title: "Unable to fetch document",
            text: `While retrieving ${id}, got a fatal error.`,
          },
          60000
        );
      }
    }
    if (to.id !== from?.id || to.hash !== from?.hash) {
      await nextTick();
      let container = contentEl.value;
      while (window.getComputedStyle(container).position === "static") {
        container = container.parentNode;
      }
      if (to.hash === "") {
        container.scrollTo(0, 0);
      } else {
        const el = document.querySelector(to.hash);
        if (el !== null) {
          container.scrollTo(0, el.offsetTop);
        }
      }
    }
  },
  { immediate: true }
);
</script>
