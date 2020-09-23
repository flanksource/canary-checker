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
    },
    calcTooltipId() {
      return (mergedDesc, name, type) => {
        return window.btoa(mergedDesc + name + type)
      }
    },
  },
  methods: {
    ...Vuex.mapActions(['pauseAutoUpdate', 'resumeAutoUpdate']),
    async triggerMerged(checks, event) {
      const btn = event.currentTarget
      btn.classList.toggle("btn-light")
      await this.$store.dispatch('triggerMergedChecks', checks)
      await this.$store.dispatch('fetchData')
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
      placement="top"
      :delay="{ show: 50, hide: 350 }"
      @show="onShow">
    <template v-slot:title><div class="description">{{description}}</div><div>{{elapsed}}</div></template>
    <template v-slot:default>
        <div>{{checkStatus.message}}</div>
        <div class="duration">Duration: {{checkStatus.duration / 1000}}s <br/>{{dateTime}}</div>
        <hr/>
        <div class="left health">Avg latency: {{health.latency}}</br>Uptime: {{health.uptime}}</div>

        <button class="btn btn-info btn-xs float-right check-button mb-2" @click="triggerCheck" title="Trigger the check on particular server">
          <i class="material-icons md-14 align-middle">loop</i>
        </button>

        <button class="btn btn-warning btn-xs float-right check-button mb-2 prometheus-graph" v-b-modal="modalName(checkStatus.key)" title="Open Prometheus graph">
          <i class="material-icons md-14 align-middle">bar_chart</i>
        </button>
    </template>
    </b-popover>
    <check-prometheus :check-type="checkType" :check-key="endpoint" :canary-name="canaryName" :target-id="modalName(checkStatus.key)"></check-prometheus>
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
    },
    description: {
      type: String,
      required: true
    },
    checkType: {
      type: String,
      required: true
    },
    canaryName: {
      type: String,
      required: true,
    },
    endpoint: {
      type: String,
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
    },
    modalName(key) {
      return "prometheus-modal-" + key
    }
  }
})


//deprecated component
Vue.component('check-tds', {
  template: `
    <transition-group name="slide" tag="section" class="check-section" :style="{width: 1.4 * check.checkStatuses[this.server].length + 'rem'}" mode="out-in">
      <div v-for="checkStatus in check.checkStatuses[this.server]" :key="checkStatus.key" class="check-status-container">
        <check-status 
            :checkStatus="checkStatus" 
            :health="check.health[server]"
            :check-type="check.type"
            :canary-name="check.canaryName"
            :endpoint="check.endpoint"
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

Vue.component('checkSetTds', {
  template: `
    <transition-group name="slide" tag="section" class="check-section" :style="{width: 1.4 * statusesSet.length + 'rem'}" mode="out-in">
      <div v-for="statusData in statusesSet" :key="statusData.checkStatus.key" class="check-status-container">
        <check-status 
            :checkStatus="statusData.checkStatus"
            :description="statusData.check.description"
            :health="statusData.check.health[server]"
            :check-type="statusData.check.type"
            :canary-name="statusData.check.canaryName"
            :endpoint="statusData.check.endpoint"
            @triggerCheck="triggerCheck(statusData.check)"
            ></check-status>
      </div>
    </transition-group>
  `,
  props: {
    checkSet: {
      type: Array,
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
    statusesSet() {
      let statusesSet = []
      let serverRelatedCount = 0
      for (const check of this.checkSet) {
        if (check.checkStatuses[this.server]) {
          serverRelatedCount += 1
          for (const checkStatus of check.checkStatuses[this.server]) {
            statusesSet.push({check, checkStatus})
          }
        }
      }

      const sorted = _.sortBy(statusesSet, function(statusData) {
        return new Date( statusData.checkStatus.time + " UTC");
      }).reverse();

      const chunked = _.chunk(sorted, serverRelatedCount * 2)

      return this.checkSet.length === 1 ? sorted : chunked[0]
    }
  },
  methods: {
    triggerCheck(check) {
      this.$store.dispatch('triggerSingleCheck', { server: this.server, check })
    }
  }
})

Vue.component('check-prometheus', {
  template: `
    <b-modal
      :id="targetId"
      size='lg'
      @show="onShow"
      custom-class="prometheus-popover">
    <template v-slot:modal-title><div class="description">Prometheus Graph <span class="badge badge-danger">{{ checkType }}</span> <span class="badge badge-secondary">{{ checkKey }}</span></div></template>
      <div class="btn-group" role="group" aria-label="Timeframe">
        <button v-for="ts in timeSelector" type="button" :class="btnClass(ts.value)" v-on:click="setSelector(ts.value)">{{ ts.name }}</button>
      </div>

      <line-chart name="Success" field="success" :check-type="checkType" :check-key="checkKey" :canary-name="canaryName" :time-selector="currentSelector" :key="currentSelector" :styles="chartStyle"></line-chart>
      <hr/>

      <line-chart name="Failed" field="failed" :check-type="checkType" :check-key="checkKey" :canary-name="canaryName" :time-selector="currentSelector" :key="currentSelector" :styles="chartStyle"></line-chart>
      <hr/>

      <line-chart name="Latency" field="latency" :check-type="checkType" :check-key="checkKey" :canary-name="canaryName" :time-selector="currentSelector" :key="currentSelector" :styles="chartStyle"></line-chart>
      <hr/>

    </b-modal>
  `,
  data() {
    return {
      elapsed: null,
      dateTime: null,
      timeSelector: [],
      currentSelector: 3600,
      successLabels: [],
      successValues: [],
      failedLabels: [],
      failedValues: [],
      latencyLabels: [],
      latencyValues: [],
    }
  },
  props: {
    checkKey: {
      type: String,
      required: true,
    },
    checkType: {
      type: String,
      required: true,
    },
    canaryName: {
      type: String,
      required: true,
    },
    targetId: {
      type: String,
      required: true,
    }
  },
  computed: {
    chartStyle() {
      return {
        width: `750px`,
      }
    }
  },
  methods: {
    btnClass(value) {
      if (value == this.currentSelector) {
        return "btn btn-danger"
      }
      return "btn"
    },
    setSelector(value) {
      this.currentSelector = value
    },
    onShow() {
      console.log(this.targetId)
      this.timeSelector = [
        {name: "1H", value: 3600},
        {name: "3H", value: 3600 * 3},
        {name: "6H", value: 3600 * 6},
        {name: "12H", value: 3600 * 12},
        {name: "1D", value: 3600 * 24},
        {name: "3D", value: 3600 * 24 * 3},
        {name: "1W", value: 3600 * 24 * 7},
      ]
    },
  }
})

Vue.component('line-chart', {
  extends: VueChartJs.Line,
  mixins: [VueChartJs.mixins],
  props: ['options'],
  async created() {
    await this.fetchData(this.timeSelector)
  },
  data() {
    return {
      seriesLabels: [],
      seriesValues: [],
      datacollection: null,
      options: {
        maintainAspectRatio: false,
        scales: {
          yAxes: [{
              id: 'Value',
              type: 'linear',
              offset: true,
              ticks: {
                min: 0,
              },
          }],
          xAxes: [{
            id: 'Time',
            ticks: {
              maxRotation: 0,
              autoSkipPadding: 30,
            },
          }]
        },
      },
    }
  },
  props: {
    name: {
      type: String,
      required: true,
    },
    checkKey: {
      type: String,
      required: true,
    },
    checkType: {
      type: String,
      required: true,
    },
    canaryName: {
      type: String,
      required: true,
    },
    field: {
      type: String,
      required: true,
    },
    timeSelector: {
      type: Number,
      required: true
    }
  },
  methods: {
    fillData () {
      this.datacollection = {
        labels: this.seriesLabels,
        datasets: [
          {
            borderColor: "#dc3545",
            fill: false,
            cubicInterpolationMode: 'monotone',
            label: this.name,
            backgroundColor: '#f87979',
            data: this.seriesData,
          }
        ]
      }
    },
    fetchData() {
      axios
        .post('/api/prometheus/graph', { checkType: this.checkType, canaryName: this.canaryName, checkKey: this.checkKey, timeframe: this.timeSelector})
        .then((response) => {
          data = response.data[this.field]
          this.seriesLabels = data.map(x => this.formatLabel(x.time))
          this.seriesData = data.map(x => this.formatValue(x.value))
          this.fillData()
          this.renderChart(this.datacollection, this.options)
        })
        .catch((err) => {
          if (err.response === undefined) {
            console.log("Error: " + err)
          } else if (err.response.status === 0) {
            console.log("Error loading data from server: failed to connect to sercer")
          } else {
            console.log("Error loading data from server: failed: " + err.response.data)
          }
        })
    },
    formatLabel(label) {
      if (this.currentSelector > (3600 * 24)) {
        return moment(label * 1000).format("D/M HH:mm")
      }
      return moment(label * 1000).format("HH:mm")
    },
    formatValue(value) {
      return Math.round(parseFloat(value, 10))
    },
  }
})