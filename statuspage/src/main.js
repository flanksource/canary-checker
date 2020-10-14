import Vue from 'vue'
import Vuex from 'vuex'
import { BootstrapVue, IconsPlugin, BIconPause, BIconPlay } from 'bootstrap-vue'


Vue.use(BootstrapVue)
Vue.use(IconsPlugin)
Vue.component('BIconPause', BIconPause )
Vue.component('BIconPlay', BIconPlay )


import PauseIcon from 'vue-material-design-icons/Menu.vue';

Vue.component('pause-icon', PauseIcon);

import 'bootstrap/dist/css/bootstrap.css'
import 'bootstrap-vue/dist/bootstrap-vue.css'
import 'bootstrap-vue/dist/bootstrap-vue-icons.css'

import store from './store'
import App from './App.vue'

Vue.use(Vuex)
Vue.use(store)


Vue.config.productionTip = false

new Vue({
    el: '#app',
    store: store,
    render: h => h(App),
})
