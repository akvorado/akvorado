import { createRouter, createWebHistory } from "vue-router";
import HomePage from "@/views/HomePage.vue";
import VisualizePage from "@/views/VisualizePage.vue";
import DocumentationPage from "@/views/DocumentationPage.vue";
import NotFoundPage from "@/views/NotFoundPage.vue";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", name: "Home", component: HomePage, meta: { title: "Home" } },
    {
      path: "/visualize",
      name: "Visualize",
      component: VisualizePage,
      meta: { title: "Visualize" },
    },
    {
      path: "/visualize/:state",
      name: "VisualizeWithState",
      component: VisualizePage,
      meta: { title: "Visualize" },
    },
    { path: "/docs", redirect: "/docs/intro" },
    {
      path: "/docs/:id",
      name: "Documentation",
      component: DocumentationPage,
      meta: { title: "Documentation" },
    },
    {
      path: "/:pathMatch(.*)",
      name: "404",
      component: NotFoundPage,
      meta: { title: "Not found" },
    },
  ],
});

export default router;
