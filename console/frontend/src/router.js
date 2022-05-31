import { createRouter, createWebHistory } from "vue-router";
import HomePage from "@/views/HomePage.vue";
import VisualizePage from "@/views/VisualizePage.vue";
import DocumentationPage from "@/views/DocumentationPage.vue";
import ErrorPage from "@/views/ErrorPage.vue";

const checkAuthenticated = async (to, from, next) => {
  const response = await fetch("/api/v0/console/user/info");
  if (response.status == 401) {
    next({ name: "401", query: { redirect: to.path } });
  } else {
    next();
  }
};

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
      name: "Home",
      component: HomePage,
      meta: { title: "Home" },
      beforeEnter: [checkAuthenticated],
    },
    {
      path: "/visualize",
      name: "Visualize",
      component: VisualizePage,
      meta: { title: "Visualize" },
      beforeEnter: [checkAuthenticated],
    },
    {
      path: "/visualize/:state",
      name: "VisualizeWithState",
      component: VisualizePage,
      meta: { title: "Visualize" },
      props: (route) => ({ routeState: route.params.state }),
      beforeEnter: [checkAuthenticated],
    },
    {
      path: "/docs",
      redirect: "/docs/intro",
      beforeEnter: [checkAuthenticated],
    },
    {
      path: "/docs/:id",
      name: "Documentation",
      component: DocumentationPage,
      meta: { title: "Documentation" },
      props: true,
      beforeEnter: [checkAuthenticated],
    },
    {
      path: "/:pathMatch(.*)",
      name: "404",
      component: ErrorPage,
      meta: { title: "Not found" },
      props: { error: "Not found!" },
    },
    {
      path: "/login",
      name: "401",
      component: ErrorPage,
      meta: { title: "Not authorized" },
      props: { error: "Not authorized!" },
    },
  ],
});

export default router;
