var app = new Vue({
  el: '#app',
  store,
  created() {
    this.$store.dispatch('fetchData')
    this.$store.dispatch('resumeAutoUpdate')
  },
  data() {
    return {
      descLimit: 41,
      nsLimit: 31
    }
  },
  computed: {
    ...Vuex.mapState(['error','servers','lastRefreshed','checks','disableReload']),
    ...Vuex.mapGetters(['serversByNames','groupedChecks']),
    shortHand() {
      return (txt, limit) => {
        return txt.slice(0, limit) + (txt.length > limit ? "..." : "");
      }
    }
  },
  methods: {
    ...Vuex.mapActions(['pauseAutoUpdate', 'resumeAutoUpdate']),
    async triggerCheck(check, event) {
      const btn = event.currentTarget
      btn.classList.toggle("btn-light")
      await this.$store.dispatch('triggerCheckOnAllServers', {check})
      btn.classList.toggle("btn-light")
    }
  }
})

Vue.component('checkStatus', {
  template: `
  <div class="check-status" :class="[checkStatus.status ? 'check-status-pass' : 'check-status-fail']" :id="checkStatus.key">
    <b-popover 
      :target="checkStatus.key" 
      triggers="hover" 
      placement="auto"
      :delay="{ show: 50, hide: 350 }"
      @show="onShow">
    <template v-slot:title>{{elapsed}}</template>
    <template v-slot:default>
        <div>{{checkStatus.message}}</div>
        <div class="duration">Duration: {{checkStatus.duration / 1000}}s <br/>{{dateTime}}</div>
        <hr/>
        <div class="left health">Avg latency: {{health.latency}}</br>Uptime: {{health.uptime}}</div>
        <button class="btn btn-info btn-xs float-right check-button mb-2" @click="triggerCheck" title="Trigger the check on particular server">
          <i class="material-icons md-14 align-middle">loop</i>
        </button>
    </template>
  </b-popover>
  </div>
  `,
  data() {
    return {
      elapsed: null,
      dateTime: null
    }
  },
  props: {
    checkStatus: {
      type: Object,
      required: true
    },
    health: {
      type: Object,
      required: true
    }
  },
  methods: {
    onShow() {
      const dateTime = new Date( this.checkStatus.time + " UTC");
      let t = new timeago()
      this.elapsed = t.simple(date.format(dateTime, 'YYYY-MM-DD HH:mm:ss', false), 'en_US')
      this.dateTime = moment(dateTime).format()
    },
    triggerCheck() {
      this.$root.$emit('bv::hide::popover')
      // this.$refs.popover.$emit('close')
      this.$emit('triggerCheck')
    }
  }
})

Vue.component('check-tds', {
  template: `
    <transition-group name="slide" tag="section" class="check-section" :style="{width: 1.4 * check.checkStatuses[this.server].length + 'rem'}" mode="out-in">
      <div v-for="checkStatus in check.checkStatuses[this.server]" :key="checkStatus.key" class="check-status-container">
        <check-status 
            :checkStatus="checkStatus" 
            :health="check.health[server]"
            @triggerCheck="triggerCheck"
            ></check-status>
      </div>
    </transition-group>
  `,
  props: {
    check: {
      type: Object,
      required: true,
    },
    server: {
      type: String,
      required: true,
    }
  },
  computed: {
    ...Vuex.mapState(['servers']),
    ...Vuex.mapGetters(['serversByNames']),
  },
  methods: {
    triggerCheck() {
      this.$store.dispatch('triggerSingleCheck', { server: this.server, check: this.check })
    }
  }
})