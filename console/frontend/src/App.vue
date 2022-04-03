<template>
  <Notifications />
  <Navigation />
  <main>
    <router-view />
  </main>
</template>

<script setup>
 import './tailwind.css';

 import Navigation from "./components/Navigation.vue";
 import Notifications from "./components/Notifications.vue";
</script>
