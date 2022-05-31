<template>
  <ThemeProvider>
    <TitleProvider>
      <router-view v-slot="{ Component }">
        <UserProvider>
          <div class="flex h-full max-h-screen flex-col">
            <NavigationBar class="flex-none" />
            <main class="relative flex grow overflow-y-auto">
              <component :is="Component" />
            </main>
          </div>
        </UserProvider>
      </router-view>
    </TitleProvider>
  </ThemeProvider>
</template>

<script setup>
import "./tailwind.css";

import NavigationBar from "@/components/NavigationBar.vue";
import TitleProvider from "@/components/TitleProvider.vue";
import ThemeProvider from "@/components/ThemeProvider.vue";
import UserProvider from "@/components/UserProvider.vue";
</script>
