import Vue from 'vue'

// using Vuex for state management
import Vuex from 'vuex'
Vue.use(Vuex)

// using Lodash
import VueLodash from 'vue-lodash'
import lodash from 'lodash'
Vue.use(VueLodash, {  lodash: lodash })

// using Bootstrap Vue
import BootstrapVue from 'bootstrap-vue'
Vue.use(BootstrapVue)
import 'bootstrap/dist/css/bootstrap.css'
import 'bootstrap-vue/dist/bootstrap-vue.css'


// using Material Design Icons
import PauseIcon from 'vue-material-design-icons/Pause.vue';
import PlayIcon from 'vue-material-design-icons/Play.vue';
import SendIcon from 'vue-material-design-icons/Send.vue';
import CharBarIcon from 'vue-material-design-icons/ChartBar.vue';
import SyncIcon from 'vue-material-design-icons/Sync.vue';
Vue.component('pause-icon', PauseIcon);
Vue.component('play-icon', PlayIcon)
Vue.component('send-icon', SendIcon);
Vue.component('chart-bar-icon', CharBarIcon);
Vue.component('sync-icon', SyncIcon);
import 'vue-material-design-icons/styles.css';

import App from './App.vue'
import store from './store'

Vue.config.productionTip = false

new Vue({
    el: '#app',
    store: store,
    render: h => h(App),
})