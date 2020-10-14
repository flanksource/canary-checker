<template>
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
</template>

<script>
  import Vuex from 'vuex'
  import _ from 'lodash';
  import CheckStatus from './CheckStatus.vue'


  export default {
  name: 'CheckSetTds',
  components: {
    CheckStatus
  },
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
}
</script>

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>
/*todo: anything here? */
</style>
