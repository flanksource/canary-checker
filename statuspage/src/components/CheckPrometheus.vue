<template>
  <b-modal
          :id="targetId"
          size='lg'
          @show="onShow"
          custom-class="prometheus-popover">
    <template v-slot:modal-title><div class="description">Prometheus Graph <span class="badge badge-danger">{{ checkType }}</span> <span class="badge badge-secondary">{{ checkKey }}</span></div></template>
    <div class="btn-group" role="group" aria-label="Timeframe">
      <button v-for="ts in timeSelector" type="button" :class="btnClass(ts.value)" v-on:click="setSelector(ts.value) " :key="ts.id" >{{ ts.name }}</button>
    </div>

    <line-chart name="Success" field="success" :check-type="checkType" :check-key="checkKey" :canary-name="canaryName" :time-selector="currentSelector" :key="currentSelector" :styles="chartStyle"></line-chart>
    <hr/>

    <line-chart name="Failed" field="failed" :check-type="checkType" :check-key="checkKey" :canary-name="canaryName" :time-selector="currentSelector" :key="currentSelector" :styles="chartStyle"></line-chart>
    <hr/>

    <line-chart name="Latency" field="latency" :check-type="checkType" :check-key="checkKey" :canary-name="canaryName" :time-selector="currentSelector" :key="currentSelector" :styles="chartStyle"></line-chart>
    <hr/>

  </b-modal>
</template>

<script>
 // import Vuex from 'vuex'


  export default {
    name: 'CheckPrometheus',
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

}
</script>

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>
/*todo: anything here? */
</style>
