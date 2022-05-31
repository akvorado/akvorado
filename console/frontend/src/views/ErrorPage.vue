<template>
  <div class="container flex flex-col items-center justify-center">
    <img
      class="block hover:animate-none motion-safe:animate-bounce"
      src="@/assets/images/akvorado.svg"
    />
    <h1 class="text-5xl font-bold">{{ error }}</h1>
  </div>
</template>

<script setup>
defineProps({
  error: {
    type: String,
    required: true,
  },
});
</script>
