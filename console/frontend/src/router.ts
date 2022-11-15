// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import { createRouter, createWebHistory } from "vue-router";
import HomePage from "@/views/HomePage.vue";
import VisualizePage from "@/views/VisualizePage.vue";
import DocumentationPage from "@/views/DocumentationPage.vue";
import ErrorPage from "@/views/ErrorPage.vue";

declare module "vue-router" {
  interface RouteMeta {
    title: string;
    notAuthenticated?: boolean;
  }
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
      name: "Home",
      component: HomePage,
      meta: { title: "Home" },
    },
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
      props: (route) => ({ routeState: route.params.state }),
    },
    {
      path: "/docs",
      redirect: "/docs/intro",
    },
    {
      path: "/docs/:id",
      name: "Documentation",
      component: DocumentationPage,
      meta: { title: "Documentation" },
      props: true,
    },
    {
      path: "/:pathMatch(.*)",
      name: "404",
      component: ErrorPage,
      meta: { title: "Not found", notAuthenticated: true },
      props: { error: "Not found!" },
    },
    {
      path: "/login",
      name: "401",
      component: ErrorPage,
      meta: { title: "Not authorized", notAuthenticated: true },
      props: { error: "Not authorized!" },
    },
  ],
});

export default router;
