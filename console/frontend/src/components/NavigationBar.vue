<template>
  <Disclosure
    v-slot="{ open }"
    as="nav"
    class="z-40 w-full border-gray-200 bg-gradient-to-r from-blue-100 to-indigo-200 px-2 py-2.5 shadow dark:from-gray-800 dark:to-gray-600 dark:shadow-white/10 sm:px-4"
  >
    <div class="container mx-auto flex flex-wrap items-center justify-between">
      <router-link to="/" class="flex items-center">
        <img
          src="@/assets/images/akvorado.svg"
          class="mr-3 h-6 sm:h-9"
          alt="Akvorado Logo"
        />
        <span
          class="self-center whitespace-nowrap text-xl font-semibold dark:text-white"
          >Akvorado</span
        >
      </router-link>
      <div class="flex md:order-2">
        <DarkModeSwitcher />
        <DisclosureButton
          class="ml-3 inline-flex items-center rounded-lg p-2 text-sm text-gray-500 hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-300 dark:text-gray-400 dark:hover:bg-gray-700 dark:focus:ring-blue-800 md:hidden"
        >
          <span class="sr-only">Open main menu</span>
          <MenuIcon v-if="!open" class="h-6 w-6" />
          <XIcon v-else class="h-6 w-6" />
        </DisclosureButton>
      </div>
      <DisclosurePanel
        static
        class="w-full items-center justify-between md:order-1 md:block md:w-auto"
        :class="open ? 'block' : 'hidden'"
      >
        <ul
          class="mt-4 flex flex-col md:mt-0 md:flex-row md:space-x-8 md:text-sm md:font-medium"
        >
          <li v-for="item in navigation" :key="item.name">
            <router-link
              class="block rounded py-2 pr-4 pl-3 focus:outline-none focus:ring-2 focus:ring-blue-300 dark:focus:ring-blue-800"
              :class="
                item.current
                  ? [
                      'bg-blue-700 text-white dark:text-white md:bg-transparent md:text-blue-700',
                    ]
                  : [
                      'text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-white md:hover:text-blue-700 md:dark:hover:text-white',
                    ]
              "
              :to="item.link"
              :aria-current="item.current && 'page'"
            >
              <component :is="item.icon" class="inline h-5 w-5"></component>
              {{ item.name }}
            </router-link>
          </li>
        </ul>
      </DisclosurePanel>
    </div>
  </Disclosure>
</template>

<script setup>
import { computed } from "vue";
import { useRoute } from "vue-router";
import { Disclosure, DisclosureButton, DisclosurePanel } from "@headlessui/vue";
import {
  HomeIcon,
  BookOpenIcon,
  MenuIcon,
  XIcon,
  PresentationChartLineIcon,
} from "@heroicons/vue/solid";
import DarkModeSwitcher from "@/components/DarkModeSwitcher.vue";

const route = useRoute();
const navigation = computed(() => [
  { name: "Home", icon: HomeIcon, link: "/", current: route.path == "/" },
  {
    name: "Visualize",
    icon: PresentationChartLineIcon,
    link: "/visualize",
    current: route.path.startsWith("/visualize"),
  },
  {
    name: "Documentation",
    icon: BookOpenIcon,
    link: "/docs",
    current: route.path.startsWith("/docs"),
  },
]);
</script>
