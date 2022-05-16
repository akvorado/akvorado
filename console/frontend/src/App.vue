<template>
  <ThemeProvider>
    <div class="flex h-full max-h-screen flex-col">
      <NavigationBar class="flex-none" />
      <main class="relative flex grow overflow-y-auto">
        <router-view />
      </main>
    </div>
  </ThemeProvider>
</template>

<script setup>
import "./tailwind.css";

import NavigationBar from "./components/NavigationBar.vue";
import ThemeProvider from "./components/ThemeProvider.vue";
</script>
