<template>
  <NotificationGroup group="top">
    <div class="fixed z-50 inset-0 flex items-start justify-center p-6 px-4 py-6 pointer-events-none">
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
          move-delay="delay-300">
          <div
            class="flex w-full max-w-sm mx-auto mt-4 overflow-hidden bg-stone-100 dark:bg-stone-800 rounded-lg shadow-md pointer-events-auto"
            v-for="notification in notifications"
            :key="notification.id">
            <div
              class="flex w-6"
              :class="{'bg-red-500': notification.kind === 'error',
                'bg-green-500': notification.kind === 'success',
                'bg-blue-500': notification.kind === 'info'}">&nbsp;
            </div>
            <div class="ml-3 w-0 flex-1 py-4">
              <p class="font-semibold text-gray-800 dark:text-gray-300">{{ notification.title }}</p>
              <p class="text-sm">{{ notification.text }}</p>
            </div>
            <div class="flex flex-shrink-0 ml-4 p-2 h-full">
              <button
                @click="close(notification.id)"
                class="inline-flex text-gray-400 rounded-md hover:text-gray-500 focus:outline-none focus:ring-2 focus:ring-gray-200 dark:focus:ring-gray-600">
                <span class="sr-only">Close</span>
                <XIcon class="w-5 h-5" />
              </button>
            </div>
          </div>
        </Notification>
      </div>
    </div>
  </NotificationGroup>
</template>

<script setup>
 import { XIcon } from '@heroicons/vue/solid';
</script>
