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
      nsLimit: 31,
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

// A graphical strip representing status info
Vue.component('status-strip', {
  template: `
    <div class="status-strip" >
        <check-time class="time-left" :time="latest"/>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          style="text-wrap: normal;"
          baseProfile="tiny"
          version="1.2"
          :width="fullWidth"
          :height="barMaxHeight">
          <g  
            v-for="(bar, index) in barSet" 
            :id="'bar-'+barSet[index].key">
            <!-- This rect is not for visual effect,-->
            <!-- but makes the following actual     -->
            <!-- data bar easier to select when it  -->
            <!-- is narrow.                         -->
            <rect  
              :height="barMaxHeight" :width="barWidth" 
              :x="barSet[index].x"  
              :style=" {fill: 'white'}"/>
            <rect                   
              :height="barSet[index].height" :width="barWidth" 
              :x="barSet[index].x" :y="barSet[index].y" 
              :style=" {fill: barSet[index].color}"/>
          </g>
        </svg>
        <check-time class="time-right" :time="earliest"/>
        <bar-popover 
          v-for="bar in barSet" 
          :target="'bar-'+bar.key"  
          :checkStatusKey="bar.key"
          :description="bar.description"
          :time="bar.time" :duration="bar.duration"
          :message="bar.message"
          :health="bar.health"/>
        <check-prometheus
            v-for="bar in barSet"
            :checkType="bar.checkType"
            :check-key="bar.endpoint"
            :canary-name="bar.canaryName"
            :target-id="modalName(bar.key)"></check-prometheus>
    </div>`,
  props: {
    checkSet: {
      type: Array,
      required: true,
    },
    server: {
      type: String,
      required: true,
    },
    color: {
      type: String,
      default: 'green',
      required: false,
    },
    errorColor: {
      type: String,
      default: 'red',
      required: false,
    },
    barWidth: {
      type: Number,
      default: 200,
      required: false,
    },
    // When variances are small they are hard to
    // see: a zoominess of 0 does no zooming,
    //      a zoominess of 1 shows only the
    //      variances by chopping off the
    //      common minimum value.
    zoominess: {
      type: Number,
      default: 0,
      required: false,
    },
    barMaxHeight: {
      type: Number,
      default: 20,
      required: false,
    },
    barSpacing: {
      type: Number,
      default: 50,
      required: false,
    },
  },
  computed: {
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

      const sorted = _.sortBy(statusesSet, function (statusData) {
        return new Date(statusData.checkStatus.time + " UTC");
      }).reverse();

      const chunked = _.chunk(sorted, serverRelatedCount * 2)

      return this.checkSet.length === 1 ? sorted : chunked[0]
    },
    fullWidth() {
      return (this.barWidth + this.barSpacing) * this.statusesSet.length
    },
    barSet() {
      let barSet = []
      //first find the minimum and maximum durations (skipping non-durations)
      //to be able to scale the data to fit in the allocated height
      let maxDuration = null
      let minDuration = null
      for (const statusData of this.statusesSet) {
        if (!statusData.checkStatus.status) {
          continue
        }
        if (maxDuration === null || statusData.checkStatus.duration > maxDuration) {
          maxDuration = statusData.checkStatus.duration
        }
        if (minDuration === null || statusData.checkStatus.duration < minDuration) {
          minDuration = statusData.checkStatus.duration
        }
      }

      let i = 0
      for (const statusData of this.statusesSet) {
        //scale the duration based on the minimum, maximum and zoominess
        if (statusData.checkStatus.status) {
          offsetDuration = statusData.checkStatus.duration - minDuration * this.zoominess
          scaledDuration = offsetDuration / (maxDuration - minDuration * this.zoominess)
          normalizedDuration = scaledDuration * this.barMaxHeight
          if (normalizedDuration < 0.5) {
            // show at least a sliver for the minimum value
            normalizedDuration = 0.5
          }
        } else {
          // for an invalid sample we show a full bar
          normalizedDuration = this.barMaxHeight
        }

        let bar = {
          "key": statusData.checkStatus.key,
          "width": this.barWidth,
          "height": statusData.checkStatus.status ? normalizedDuration : this.barMaxHeight,
          "x": (this.barWidth + this.barSpacing) * i,
          "y": (this.barMaxHeight - normalizedDuration),
          "color": statusData.checkStatus.status ? this.color : this.errorColor,
          "checkStatus": statusData.checkStatus,
          "description": statusData.check.description,
          "message": statusData.checkStatus.message,
          "health": statusData.check.health[this.server],
          "duration": statusData.checkStatus.duration,
          "time": statusData.checkStatus.time,
          "checkType": statusData.check.type,
          "endpoint": statusData.check.endpoint,
          "canaryName": statusData.check.canaryName,
        }
        barSet.push(bar);
        i++
      }
      return barSet;
    },
    latest() {
      var latestSoFar = null;
      for (const statusData of this.statusesSet) {
        const checkDate = new Date(statusData.checkStatus.time + " UTC");
        if (latestSoFar === null || checkDate > latestSoFar) {
          latestSoFar = checkDate
        }
      }
      var ta = timeago();
      return ta.ago(latestSoFar, true)
    },
    earliest() {
      var earliestSoFar = null;
      for (const statusData of this.statusesSet) {
        const checkDate = new Date(statusData.checkStatus.time + " UTC");
        if (earliestSoFar === null || checkDate < earliestSoFar) {
          earliestSoFar = checkDate
        }
      }
      var ta = timeago();
      return ta.ago(earliestSoFar, true)
    },
  },
  methods: {
    modalName(key) {
      return "prometheus-modal-" + key
    }
  }
})

// A small textual display that serves as book-end
// for check info
Vue.component('check-time', {
  props: ['time'],
  template: '<div style="font-size: xx-small;">{{ time }}</div>'
})

// This component encapsulates a pop-over displayed
// when hovering of a check infographic
Vue.component('bar-popover', {
  template: `
   <b-popover 
        :target="target"  
        triggers="hover" 
        placement="top"
        :delay="{ show: 50, hide: 350 }" 
        @show="onShow">
        <template v-slot:title>
            <div class="description">{{description}}</div><div>{{elapsed}}</div>
        </template>
        <template v-slot:default>
          <div>{{message}}</div>
          <div class="duration">Duration: {{duration / 1000}}s <br/>{{dateTime}}</div>
          <hr/>
          <div class="left health">Avg latency: {{health.latency}}</br>Uptime: {{health.uptime}}</div>
          <button class="btn btn-info btn-xs float-right check-button mb-2" @click="triggerCheck" title="Trigger the check on particular server">
            <i class="material-icons md-14 align-middle">loop</i>
          </button>
          <button class="btn btn-warning btn-xs float-right check-button mb-2 prometheus-graph" v-b-modal="modalName(checkStatusKey)" title="Open Prometheus graph">
             <i class="material-icons md-14 align-middle">bar_chart</i>
          </button>
        </template>
    </b-popover>`,
  data() {
    return {
      elapsed: null,
      dateTime: null
    }
  },
  props: {
    target: {
      type: String,
      required: true
    },
    checkStatusKey: {
      type: String,
      required: true
    },
    description: {
      type: String,
      required: true
    },
    message: {
      type: String,
      required: true
    },
    time: {
      type: Object,
      required: true
    },
    duration: {
      type: Number,
      required: true
    },
    health: {
      type: Object,
      required: true
    },
  },
  methods: {
    onShow() {
      const dateTime = new Date(this.time + " UTC");
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

// From: https://github.com/digplan/time-ago/blob/master/timeago.js
// License: MIT Copyright (c) 2015 Chris Borkert
// https://github.com/digplan/time-ago/blob/master/license.txt
var timeago = function() {

  var o = {
    second: 1000,
    minute: 60 * 1000,
    hour: 60 * 1000 * 60,
    day: 24 * 60 * 1000 * 60,
    week: 7 * 24 * 60 * 1000 * 60,
    month: 30 * 24 * 60 * 1000 * 60,
    year: 365 * 24 * 60 * 1000 * 60
  };
  var obj = {};

  obj.ago = function(nd, s) {
    var r = Math.round,
        dir = ' ago',
        pl = function(v, n) {
          return (s === undefined) ? n + ' ' + v + (n > 1 ? 's' : '') + dir : n + v.substring(0, 1)
        },
        ts = Date.now() - new Date(nd).getTime(),
        ii;
    if( ts < 0 )
    {
      ts *= -1;
      dir = ' from now';
    }
    for (var i in o) {
      if (r(ts) < o[i]) return pl(ii || 'm', r(ts / (o[ii] || 1)))
      ii = i;
    }
    return pl(i, r(ts / o[i]));
  }

  obj.today = function() {
    var now = new Date();
    var Weekday = new Array("Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday");
    var Month = new Array("January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December");
    return Weekday[now.getDay()] + ", " + Month[now.getMonth()] + " " + now.getDate() + ", " + now.getFullYear();
  }

  obj.timefriendly = function(s) {
    var t = s.match(/(\d).([a-z]*?)s?$/);
    return t[1] * eval(o[t[2]]);
  }

  obj.mintoread = function(text, altcmt, wpm) {
    var m = Math.round(text.split(' ').length / (wpm || 200));
    return (m || '< 1') + (altcmt || ' min to read');
  }

  return obj;
}