import { createRouter, createWebHistory } from "vue-router";
import HomePage from "../views/HomePage.vue";
import DocumentationPage from "../views/DocumentationPage.vue";
import NotFoundPage from "../views/NotFoundPage.vue";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", name: "Home", component: HomePage },
    { path: "/docs", redirect: "/docs/intro" },
    { path: "/docs/:id", name: "Documentation", component: DocumentationPage },
    { path: "/:pathMatch(.*)", component: NotFoundPage },
  ],
});

export default router;
