import Vue from 'vue'
import Vuex from 'vuex'
import { BootstrapVue } from 'bootstrap-vue'
import VueLodash from 'vue-lodash'
import lodash from 'lodash'



import PauseIcon from 'vue-material-design-icons/Pause.vue';
import PlayIcon from 'vue-material-design-icons/Play.vue';
import SendIcon from 'vue-material-design-icons/Send.vue';

Vue.component('pause-icon', PauseIcon);
Vue.component('play-icon', PlayIcon)
Vue.component('send-icon', SendIcon);

import 'bootstrap/dist/css/bootstrap.css'
import 'bootstrap-vue/dist/bootstrap-vue.css'

import store from './store'
import App from './App.vue'

Vue.use(Vuex)
Vue.use(store)
Vue.use(BootstrapVue)
Vue.use(VueLodash, {  lodash: lodash })


Vue.config.productionTip = false

new Vue({
    el: '#app',
    store: store,
    render: h => h(App),
})
