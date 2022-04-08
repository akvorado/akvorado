<template>
  <AppNotifications />
  <div class="flex max-h-screen flex-col">
    <NavigationBar class="flex-none" />
    <main class="relative flex grow overflow-y-auto">
      <router-view />
    </main>
  </div>
</template>

<script setup>
import "./tailwind.css";

import NavigationBar from "./components/NavigationBar.vue";
import AppNotifications from "./components/AppNotifications.vue";
</script>
