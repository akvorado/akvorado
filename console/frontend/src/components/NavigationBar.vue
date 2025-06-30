<!-- SPDX-FileCopyrightText: 2022 Free Mobile -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

<template>
  <Disclosure
    v-slot="{ open }"
    as="nav"
    class="z-40 w-full border-gray-200 bg-linear-to-r from-blue-100 to-indigo-200 px-2 py-2.5 shadow dark:from-gray-800 dark:to-gray-600 dark:shadow-white/10 sm:px-4"
  >
    <div class="container mx-auto flex flex-wrap items-center justify-between">
      <router-link to="/" class="flex items-center">
        <img
          src="@/assets/images/akvorado.svg"
          class="mr-3 h-9"
          alt="Akvorado Logo"
        />
        <span class="self-center dark:text-white">
          <span class="block text-xl font-semibold">Akvorado</span>
          <span
            class="block max-w-[8em] overflow-hidden text-ellipsis whitespace-nowrap text-xs leading-4 text-gray-600 dark:text-gray-400"
          >
            {{ serverConfiguration?.version }}
          </span>
        </span>
      </router-link>
      <div class="flex md:order-2">
        <DarkModeSwitcher />
        <UserMenu />
        <DisclosureButton
          class="ml-3 inline-flex items-center rounded-lg p-2 text-sm text-gray-500 hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-300 dark:text-gray-400 dark:hover:bg-gray-700 dark:focus:ring-blue-800 md:hidden"
        >
          <span class="sr-only">Open main menu</span>
          <MenuIcon class="h-6 w-6" :class="{ hidden: open }" />
          <XIcon class="h-6 w-6" :class="{ hidden: !open }" />
        </DisclosureButton>
      </div>
      <DisclosurePanel
        static
        class="w-full items-center justify-between md:order-1 md:block md:w-auto"
        :class="{
          block: open,
          'hidden md:block': !open,
        }"
      >
        <ul
          class="mt-4 flex flex-col md:mt-0 md:flex-row md:space-x-8 md:text-sm md:font-medium"
        >
          <li v-for="item in navigation" :key="item.name">
            <router-link
              class="block rounded py-2 pl-3 pr-4 focus:outline-none focus:ring-2 focus:ring-blue-300 dark:focus:ring-blue-800"
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

<script lang="ts" setup>
import { computed, inject } from "vue";
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
import UserMenu from "@/components/UserMenu.vue";
import { ServerConfigKey } from "@/components/ServerConfigProvider.vue";

const serverConfiguration = inject(ServerConfigKey);
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
