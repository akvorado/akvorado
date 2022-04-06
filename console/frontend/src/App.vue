<template>
  <Notifications />
  <div class="max-h-screen flex flex-col">
    <Navigation class="flex-none" />
    <main class="relative flex grow overflow-y-auto">
      <router-view />
    </main>
  </div>
</template>

<script setup>
 import './tailwind.css';

 import Navigation from "./components/Navigation.vue";
 import Notifications from "./components/Notifications.vue";
</script>
