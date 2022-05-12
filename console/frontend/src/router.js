import { createRouter, createWebHistory } from "vue-router";
import HomePage from "./views/HomePage.vue";
import VisualizePage from "./views/VisualizePage.vue";
import DocumentationPage from "./views/DocumentationPage.vue";
import NotFoundPage from "./views/NotFoundPage.vue";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", name: "Home", component: HomePage },
    { path: "/visualize", name: "Visualize", component: VisualizePage },
    {
      path: "/visualize/:state",
      name: "VisualizeWithState",
      component: VisualizePage,
    },
    { path: "/docs", redirect: "/docs/intro" },
    { path: "/docs/:id", name: "Documentation", component: DocumentationPage },
    { path: "/:pathMatch(.*)", component: NotFoundPage },
  ],
});

export default router;
