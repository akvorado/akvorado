import { createApp } from 'vue';
import Notifications from 'notiwind'
import App from './App.vue';
import router from "./router";

createApp(App)
  .use(router)
  .use(Notifications)
  .mount('#app');
