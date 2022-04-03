<template>
    <div class="w-full flex-none md:grid md:grid-cols-3 md:gap-8">
      <div class="fixed top-0 h-full overflow-y-auto bg-gray-50 dark:bg-gray-900 dark:text-gray-200 md:mx-0 py-20 px-4 sm:px-6 md:pl-0 md:pr-8">
        <nav class="text-sm max-w-[37.5rem] mx-auto md:max-w-none md:mx-0 relative">
          <ul class="space-y-8 pl-6">
            <li v-for="document in toc" :key="document.name" class="space-y-3">
              <router-link :to="{path: document.name, hash: `#${document.headers[0].id}` }"
                      class="block font-semibold"
                      :class="{ 'text-blue-600': activeDocument === document.name,
                                'dark:text-blue-300': activeDocument === document.name,
                                'text-gray-900': activeDocument !== document.name,
                                'dark:text-gray-300': activeDocument !== document.name }">
                {{ document.headers[0].title }}
              </router-link>
              <ul class="space-y-3">
                <template v-for="header in document.headers">
                  <li v-if="header.level >= 2 && header.level <= 3"
                      :class="{'ml-2': (header.level == 2), 'ml-4': (header.level == 3)}">
                    <router-link
                        :to="{path: document.name, hash: `#${header.id}` }" class="block"
                        :class="{ 'text-blue-600': activeDocument === document.name && activeSlug === header.id,
                                  'dark:text-blue-300': activeDocument === document.name && activeSlug === header.id }">
                      {{ header.title }}
                    </router-link>
                  </li>
                </template>
              </ul>
            </li>
          </ul>
        </nav>
      </div>
      <div class="col-span-2 md:-ml-8 md:shadow-md">
        <div class="py-4 px-4 md:px-16">
          <div class="prose prose-sm md:prose-base max-w-[37.5rem] mx-auto dark:prose-invert prose-h1:border-b-2 dark:prose-h1:border-gray-700 prose-img:center prose-pre:rounded"
               v-html="markdown">
          </div>
        </div>
      </div>
    </div>
</template>

<script setup>
 import { ref, computed, watch, nextTick } from 'vue';
 import { useRoute } from 'vue-router';
 import { notify } from "notiwind";

 const route = useRoute();
 const markdown = ref('');
 const toc = ref([]);
 const activeDocument = ref(null);
 const activeSlug = computed(() => route.hash.replace(/^#/, ''));

 watch(() => ({ id: route.params.id, hash: route.hash }),
       async (to, from) => {
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
             markdown.value = "hello";
             markdown.toc = [];
             activeDocument.value = "nothing";
           }
         }
         if (to.id !== from?.id || to.hash !== from?.hash) {
           if (to.hash === "") {
             window.scrollTo(0, 0);
           } else {
             await nextTick();
             const el = document.querySelector(to.hash);
             if (el !== null) {
               window.scrollTo(0, el.getBoundingClientRect().top + window.pageYOffset);
             }
           }
         }
       }, { immediate: true });

</script>
