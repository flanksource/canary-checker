<template>
  <div class="ml-4 mr-4" id="app">
    <button v-if="disableReload" v-on:click="resumeAutoUpdate" type="button" class="btn btn-danger float-right" v-cloak>
      <i class="material-icons md-18 align-middle">play_arrow</i>
      <b-icon-play class="align-middle" style="font-size: 18px;font-weight: bold; "></b-icon-play>
      <PauseIcon/>
      <span class="align-middle">Resume auto update</span>
    </button>
    <button v-else type="button" v-on:click="pauseAutoUpdate"  class="btn btn-primary float-right">
      <i class="material-icons md-18 align-middle">pause</i>
      <b-icon-pause class="align-middle" style="font-size: 18px;font-weight: bold; "></b-icon-pause>
      <span class="align-middle">Pause auto update</span>
    </button>

    <h1>Canary Checker</h1>
    <hr>

    <div v-if="error" class="alert alert-danger" role="alert" v-cloak>
      {{ error }}
    </div>

    <table id="checks" class="table table-sm table-fixed text-nowrap" v-cloak>
      <thead>
      <th class="border-right">Type</th>
      <th class="border-right">NS/Name</th>
      <th class="border-right">Description</th>
      <th v-for="(server, serverName) in serversByNames" :key="server" class="border-right">{{ serverName }}</th>
      </thead>
<!-- TODO: KEY?     <template v-for="(typed, name) in groupedChecks" :key="name">-->
      <template v-for="(typed, name) in groupedChecks" >
        <tbody v-for="(mergedChecks, type) in typed" :key="type" class="border-bottom border-secondary">
        <tr v-for="(checkSet, mergedDesc, idx) in mergedChecks" :key="mergedDesc">
          <td v-if="idx === 0" class="align-middle border-right" :rowspan="Object.keys(mergedChecks).length">
            <img :src="'images/' + type + '.svg'" height="20px" :title="type">
          </td>
          <td v-if="idx === 0" class="align-middle border-right w-10" :rowspan="Object.keys(mergedChecks).length">
            <span class="badge badge-secondary" :id="name">{{ shortHand(name, nsLimit) }}</span>
            <b-tooltip v-if="name.length > nsLimit" :target="name" triggers="hover" variant="secondary">{{name}}</b-tooltip>
          </td>
<!--          <td class="align-middle w-25">-->
<!--              <span class="float-left w-75 pr-5" :id="calcTooltipId(mergedDesc, name, type)" :class="{'font-italic': mergedDesc.startsWith('multiple')}">-->
<!--                {{ shortHand(mergedDesc, descLimit) }}-->
<!--              </span>-->
<!--            <b-tooltip :disabled="mergedDesc.length <= descLimit" :target="calcTooltipId(mergedDesc, name, type)" triggers="hover" variant="secondary"><div class="description">{{mergedDesc}}</div></b-tooltip>-->
<!--            <b-tooltip-->
<!--                    :disabled="!mergedDesc.startsWith('multiple')"-->
<!--                    :target="calcTooltipId(mergedDesc, name, type)"-->
<!--                    triggers="hover"-->
<!--                    variant="secondary">-->
<!--              <div v-for="check in checkSet" :key="check.key" class="description">{{check.description}}</div>-->
<!--            </b-tooltip>-->
<!--            <button class="btn btn-info btn-xs float-right check-button" @click="triggerMerged(checkSet, $event)" title="Trigger the check on every server">-->
<!--              <i class="material-icons md-12 align-middle">send</i>-->
<!--            </button>-->
<!--          </td>-->
<!--          <td v-for="(server, serverName) in serversByNames" :key="server" class="align-top border-right border-left">-->
<!--            <check-set-tds :check-set="checkSet" :server="server"></check-set-tds>-->
<!--          </td>-->
        </tr>
        </tbody>
      </template>
    </table>

    <div v-if="lastRefreshed" id="last-refreshed" v-cloak>
      Last refreshed <span>{{ lastRefreshed }}</span>
    </div>
  </div>
</template>

<script>
    import Vuex from 'vuex'

// import HelloWorld from './components/HelloWorld.vue'
    import store from './store'

export default {
  name: 'App',
  // components: {
  //   HelloWorld
  // },
  store: store,
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
}
</script>

<style>
/*#app {*/
/*  font-family: Avenir, Helvetica, Arial, sans-serif;*/
/*  -webkit-font-smoothing: antialiased;*/
/*  -moz-osx-font-smoothing: grayscale;*/
/*  text-align: center;*/
/*  color: #2c3e50;*/
/*  margin-top: 60px;*/
/*}*/
body {
  padding-top: 2rem;
  padding-bottom: 2rem;
}

h3 {
  margin-top: 2rem;
}

.popover > h3 {
  margin-top: 0rem;
}

.popover-body > hr {
  margin: 0.4rem 0;
}

.popover-header > .description {
  font-size: 0.75rem;
}

.tooltip-inner > .description {
  font-size: 0.6rem;
}

.row {
  margin-bottom: 1rem;
}
.row .row {
  margin-top: 1rem;
  margin-bottom: 0;
}
[class*="col-"] {
  padding-top: 1rem;
  padding-bottom: 1rem;
  background-color: rgba(86, 61, 124, .15);
  border: 1px solid rgba(86, 61, 124, .2);
}

hr {
  margin-top: 2rem;
  margin-bottom: 2rem;
}

#last-refreshed {
  color: #777;
  font-size: 0.8em;
}

div.check-status-container {
  display: inline-block;
  vertical-align: middle;
}

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
  background-color:#dc3545;
}

.btn-group-xs > .btn, .btn-xs {
  padding: .25rem .4rem;
  font-size: .875rem;
  line-height: .75;
  border-radius: .2rem;
}

[v-cloak] {
  display: none;
}

.material-icons.md-18 { font-size: 18px }
.material-icons.md-14 { font-size: 14px }
.material-icons.md-12 { font-size: 12px }

.w-10 {
  width: 10%!important;
}

.health {
  display: inline-block;
  font-size: 0.75rem;
  line-height: 0.75rem;
  width: 11rem;
}

.group-section {
  width: 150px
}

.check-section {
  min-width: 7rem;
}

.check-section-header {
  height: 1.5rem;
}

.slide-enter {
  opacity: 0;
}

.slide-enter-active {
  transition: all 0.5s ease-out;
}

.slide-leave-active {
  transition: opacity 300ms ease-out;
}

.slide-leave {
  opacity: 0;
}

.slide-move {
  transition: all 250ms ease-in;
}

.check-button {
  transition: all 0.4s linear;
  transition-property: color, background-color, border-color
}

.check-button.prometheus-graph {
  margin-right: 10px;
}

.prometheus-popover {
  max-width: 80%;
  width: 80%;
}
</style>
