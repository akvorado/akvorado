<template>
  <NotificationGroup group="top">
    <div
      class="pointer-events-none fixed inset-0 z-50 flex items-start justify-center p-6 px-4 py-6"
    >
      <div class="w-full max-w-sm">
        <Notification
          v-slot="{ notifications, close }"
          enter="transform ease-out duration-300 transition"
          enter-from="translate-y-2 opacity-0 sm:translate-y-0 sm:translate-x-4"
          enter-to="translate-y-0 opacity-100 sm:translate-x-0"
          leave="transition ease-in duration-500"
          leave-from="opacity-100"
          leave-to="opacity-0"
          move="transition duration-500"
          move-delay="delay-300"
        >
          <div
            v-for="notification in notifications"
            :key="notification.id"
            class="pointer-events-auto mx-auto mt-4 flex w-full max-w-sm overflow-hidden rounded-lg bg-stone-100 shadow-md dark:bg-stone-800"
          >
            <div
              class="flex w-6"
              :class="{
                'bg-red-500': notification.kind === 'error',
                'bg-green-500': notification.kind === 'success',
                'bg-blue-500': notification.kind === 'info',
              }"
            >
              &nbsp;
            </div>
            <div class="ml-3 w-0 flex-1 py-4">
              <p class="font-semibold text-gray-800 dark:text-gray-300">
                {{ notification.title }}
              </p>
              <p class="text-sm">{{ notification.text }}</p>
            </div>
            <div class="ml-4 flex h-full flex-shrink-0 p-2">
              <button
                class="inline-flex rounded-md text-gray-400 hover:text-gray-500 focus:outline-none focus:ring-2 focus:ring-gray-200 dark:focus:ring-gray-600"
                @click="close(notification.id)"
              >
                <span class="sr-only">Close</span>
                <XIcon class="h-5 w-5" />
              </button>
            </div>
          </div>
        </Notification>
      </div>
    </div>
  </NotificationGroup>
</template>

<script setup>
import { XIcon } from "@heroicons/vue/solid";
</script>
