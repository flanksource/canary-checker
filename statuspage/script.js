var app = new Vue({
  el: '#app',
  store,
  created() {
    this.$store.dispatch('fetchData')
    this.$store.dispatch('resumeAutoUpdate')
  },
  computed: {
    ...Vuex.mapState(['error','servers','lastRefreshed','checks','disableReload'])
  },
  methods: {
    ...Vuex.mapActions(['pauseAutoUpdate', 'resumeAutoUpdate'])
  }
})

Vue.directive('popover', {
  bind: function bsPopoverCreate(el, binding) {
    let trigger = 'hover focus';
    if (binding.modifiers.focus || binding.modifiers.hover || binding.modifiers.click) {
      const t = [];
      if (binding.modifiers.focus) t.push('focus');
      if (binding.modifiers.hover) t.push('hover');
      if (binding.modifiers.click) t.push('click');
      trigger = t.join(' ');
    }
    // Time comes as UTC from server, timeago expects local time
    // We convert from UTC to Local date
    let dateTime = new Date($(el).attr("popover-title") + " UTC");
    let t = new timeago()
    let title = t.simple(date.format(dateTime, 'YYYY-MM-DD HH:mm:ss', false), 'en_US')
    let duration = $(el).attr("popover-duration")

    let content = `${binding.value} <div class="duration">Duration: ${duration / 1000}s <br/>${dateTime}</div>`

    $(el).popover({
      title: title,
      content: content,
      placement: binding.arg,
      trigger: trigger,
      html: binding.modifiers.html
    });
  },
  unbind(el, binding) {
    $(el).popover('dispose');
  },
});

Vue.component('check-row', {
  template: `
    <tr>
      <td scope="row" class="align-middle"> 
        <img :src="'images/' + check.type + '.svg'" height="20px" :title="check.type"></i> 
        <span class="badge badge-secondary">{{ check.name }}</span> 
        <span>{{ check.description }}</span>
      </td>
      <td v-for="serverName in servers" :key="serverName" class="align-middle border-right">
        <section v-if="check.checkStatuses[serverName]">
          <button class="btn btn-secondary btn-xs" @click="triggerSingle(serverName, check.key)">Trigger</button>
          <span class="right">{{check.health[serverName].latency}} {{check.health[serverName].uptime}}</span>
          <br />
          <div v-for="checkStatus in check.checkStatuses[serverName]" :key="checkStatus.time" class="check-status-container">
            <div v-if="checkStatus.status" class="check-status check-status-pass" v-popover:auto.html="checkStatus.message" v-bind:popover-duration="checkStatus.duration"  v-bind:popover-title="checkStatus.time"></div>
            <div v-else class="check-status check-status-fail" v-popover:auto.html="checkStatus.message" v-bind:popover-duration="checkStatus.duration" v-bind:popover-title="checkStatus.time"></div>
          </div>
        </section>
      </td>
    </tr>
  `,
  props: {
    check: {
      type: Object,
      required: true,
    }
  },
  computed: {
    ...Vuex.mapState(['servers'])
  },
  methods: {
    triggerSingle(server, key) {
      axios
        .post('/api/triggerCheck', { server, key })
        .then(() => {
          this.$store.dispatch('fetchData')
        })
        .catch((err) => {
          this.$store.commit('SET_ERROR', "Trigger error: " + err.response.data)
        })
    }
  }
})