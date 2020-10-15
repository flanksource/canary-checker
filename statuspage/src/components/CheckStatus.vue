<template>
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
        <div class="left health">Avg latency: {{health.latency}}<br/>Uptime: {{health.uptime}}</div>

        <button class="btn btn-info btn-xs float-right check-button mb-2" @click="triggerCheck" title="Trigger the check on particular server">
          <sync-icon class="material-icons md-14 align-middle" />
        </button>

        <button class="btn btn-warning btn-xs float-right check-button mb-2 prometheus-graph" v-b-modal="modalName(checkStatus.key)" title="Open Prometheus graph">
          <chart-bar-icon class="material-icons md-14 align-middle" />
        </button>
      </template>
    </b-popover>
    <check-prometheus :check-type="checkType" :check-key="endpoint" :canary-name="canaryName" :target-id="modalName(checkStatus.key)"></check-prometheus>
  </div>
</template>

<script>
 // import Vuex from 'vuex'
 import CheckPrometheus from './CheckPrometheus.vue'
 import date from 'date-and-time';
 import timeago from 'date-and-time';
 import moment from 'date-and-time';

  export default {
    name: 'CheckStatus',
    components: {
      CheckPrometheus
    },
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

}
</script>

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>
  div.check-status {
    height: 0.8em;
    width: 1rem;
    margin: 0.4rem 0.2rem;
    border-radius: 0.15rem;
  }

  div.check-status.check-status-pass {
    background-color: #28a745;
  }

  div.check-status.check-status-fail {
    background-color: #dc3545;
  }

  .health {
    display: inline-block;
    font-size: 0.75rem;
    line-height: 0.75rem;
    width: 11rem;
  }

  .check-button.prometheus-graph {
    margin-right: 10px;
  }

  .prometheus-popover {
    max-width: 80%;
    width: 80%;
  }
</style>
