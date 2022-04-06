<template>
  <Disclosure as="nav"
              v-slot="{ open }"
              class="w-full z-40 bg-gradient-to-r from-blue-100 to-indigo-200 border-gray-200 px-2 sm:px-4 py-2.5 dark:from-gray-800 dark:to-gray-600 shadow">
    <div class="container flex flex-wrap justify-between items-center mx-auto">
      <router-link to="/" class="flex items-center">
        <img src="../assets/images/akvorado.svg" class="mr-3 h-6 sm:h-9" alt="Akvorado Logo" />
        <span class="self-center text-xl font-semibold whitespace-nowrap dark:text-white">Akvorado</span>
      </router-link>
      <div class="flex md:order-2">
        <DarkMode />
        <DisclosureButton class="inline-flex items-center p-2 ml-3 text-sm text-gray-500 rounded-lg md:hidden hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-gray-200 dark:text-gray-400 dark:hover:bg-gray-700 dark:focus:ring-gray-600">
          <span class="sr-only">Open main menu</span>
          <MenuIcon v-if="!open" class="w-6 h-6" />
          <XIcon v-else class="w-6 h-6" />
        </DisclosureButton>
      </div>
      <DisclosurePanel static class="justify-between items-center w-full md:block md:w-auto md:order-1"
                       :class="open?'block':'hidden'">
        <ul class="flex flex-col mt-4 md:flex-row md:space-x-8 md:mt-0 md:text-sm md:font-medium">
          <li v-for="item in navigation" :key="item.name">
            <router-link class="block py-2 pr-4 pl-3 rounded"
                         :class="item.current?['text-white bg-blue-700 md:bg-transparent md:text-blue-700 dark:text-white']:['text-gray-700 hover:bg-gray-50 md:hover:text-blue-700 dark:text-gray-400 md:dark:hover:text-white dark:hover:bg-gray-700 dark:hover:text-white dark:border-gray-700']"
                         :to="item.link"
                         :aria-current="item.current && 'page'">
              <component :is="item.icon" class="w-5 h-5 inline"></component>
              {{ item.name }}
            </router-link>
          </li>
        </ul>
      </DisclosurePanel>
    </div>
  </Disclosure>
</template>

<script setup>
 import { computed } from 'vue';
 import { useRoute } from 'vue-router';
 import { Disclosure, DisclosureButton, DisclosurePanel } from '@headlessui/vue';
 import { HomeIcon, BookOpenIcon, MenuIcon, XIcon } from '@heroicons/vue/solid';
 import DarkMode from './DarkMode.vue';

 const route = useRoute();
 const navigation = computed(() => [
     { name: 'Home', icon: HomeIcon, link: '/', current: route.path == '/' },
     { name: 'Documentation', icon: BookOpenIcon, link: '/docs', current: route.path.startsWith('/docs') },
 ]);
</script>
