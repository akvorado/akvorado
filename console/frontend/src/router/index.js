import {createRouter, createWebHistory} from 'vue-router';
import Home from  '../views/Home.vue';
import Doc from '../views/Doc.vue';
import NotFound from '../views/NotFound.vue';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", name: "Home", component: Home },
    { path: "/docs", redirect: "/docs/intro" },
    { path: "/docs/:id", name: "Documentation", component: Doc },
    { path: "/:pathMatch(.*)", component: NotFound }
  ],
});

export default router;
