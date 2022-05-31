<template>
  <Popover class="relative px-2" as="div">
    <PopoverButton
      class="flex rounded-full bg-gray-200 focus:ring-2 focus:ring-blue-300 dark:focus:ring-blue-800"
    >
      <span class="sr-only">Open user menu</span>
      <img class="h-10 w-10 rounded-full" :src="avatarURL" alt="User avatar" />
    </PopoverButton>
    <transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="translate-y-1 opacity-0"
      enter-to-class="translate-y-0 opacity-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="translate-y-0 opacity-100"
      leave-to-class="translate-y-1 opacity-0"
    >
      <PopoverPanel
        class="absolute right-0 z-50 my-4 max-w-xs list-none divide-y divide-gray-100 rounded bg-white text-base shadow dark:divide-gray-600 dark:bg-gray-700"
      >
        <div class="py-3 px-4">
          <span class="block text-sm text-gray-900 dark:text-white">
            {{ user.name || user.email || user.user }}
          </span>
          <span
            v-if="user.name && user.email"
            class="block truncate text-sm font-medium text-gray-500 dark:text-gray-400"
          >
            {{ user.email }}
          </span>
        </div>
        <ul class="py-1">
          <li v-if="user['logout-url']">
            <a
              :href="user['logout-url']"
              class="block py-2 px-4 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-600 dark:hover:text-white"
              >Logout</a
            >
          </li>
        </ul>
      </PopoverPanel>
    </transition>
  </Popover>
</template>

<script setup>
import { inject } from "vue";
import { Popover, PopoverButton, PopoverPanel } from "@headlessui/vue";

const { user } = inject("user");
const avatarURL = "/api/v0/console/user/avatar";
</script>
