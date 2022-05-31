<template>
  <img class="mx-auto mt-10" src="@/assets/images/akvorado.svg" />
  <h1 class="mt-10 text-center text-5xl font-bold">{{ error }}</h1>
</template>

<script setup>
defineProps({
  error: {
    type: String,
    required: true,
  },
});
</script>
