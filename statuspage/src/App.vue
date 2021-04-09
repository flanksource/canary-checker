<template>

    <div class="ml-4 mr-4" id="app">
        <h1>Canary Checker</h1>
        <auto-update-settings/>

        <error-panel :error="error"/>

        <table class="table table-sm table-fixed text-nowrap" id="checks" v-cloak>
            <thead>
            <th class="border-right">Description</th>
            <th :key="server.value" class="border-right" v-for="server in orderedServers">{{ server.label }}</th>
            </thead>
           <tbody>
            <template v-for="byNamespace in groupedChecks">

                <tr :key="byNamespace.namespace">
                    <td :colspan="servers.length+1">
                          <span class="badge badge-secondary">{{ shortHand(byNamespace.namespace, nsLimit) }}</span>
                    </td>
                </tr>

                <template v-for="(check) in byNamespace.items" >
                      <canary-standard-server-row :interval="check.interval"
                                 :owner="check.owner"
                                 :severity="check.severity"
                                 :check-type="check.type"
                                 :description="check.description"
                                 :name="check.name"
                                 :namespace="check.namespace"
                                 :short-description="shortHand(checkName(check),60)"
                                 :items="check.items"
                                 :key="checkKey(check)"
                                 />
                </template>

            </template>
            </tbody>
        </table>

        <div id="last-refreshed" v-cloak v-if="lastRefreshed">
            Last refreshed <span>{{ lastRefreshed }}</span>
        </div>
        <div id="never-refreshed"  v-else>
            No data received yet
        </div>
    </div>

</template>

<script>
import Vuex from "vuex";

import store from "./store";
import AutoUpdateSettings from "./components/AutoUpdateSettings.vue";
import ErrorPanel from "./components/ErrorPanel.vue";
import CanaryStandardServerRow from "./components/CanaryStandardServerRow.vue";

export default {
  name: "App",
  components: {
    CanaryStandardServerRow,
    AutoUpdateSettings,
    ErrorPanel,
  },
  store: store,
  created() {
    this.$store.dispatch("fetchData");
    this.$store.dispatch("resumeAutoUpdate");
  },
  data() {
    return {
      descLimit: 60,
      nsLimit: 31,
    };
  },
  computed: {
    ...Vuex.mapState([
      "error",
      "servers",
      "lastRefreshed",
      "checks",
      "disableReload",
    ]),
    ...Vuex.mapGetters(["orderedServers", "groupedChecks"]),

    shortHand() {
      return (txt, limit) => {
        return txt.slice(0, limit) + (txt.length > limit ? "..." : "");
      };
    },

    checkName() {
      return (check) => {
        let name = check.description
        if (name == null || name == "") {
          name = check.name
        }
        return name
      }
    },

    checkKey() {
      return (check) => {
        return `${check.key}${check.id}${check.description}${check.name}${check.namespace}${check.endpoint}${check.serverURL}`
      }
    },

    calcTooltipId() {
      return (mergedDesc, name, type) => {
        return window.btoa(mergedDesc + name + type);
      };
    },
  },
  methods: {
    ...Vuex.mapActions(["pauseAutoUpdate", "resumeAutoUpdate"]),
    async triggerMerged(checks, event) {
      const btn = event.currentTarget;
      btn.classList.toggle("btn-light");
      await this.$store.dispatch("triggerMergedChecks", checks);
      await this.$store.dispatch("fetchData");
      btn.classList.toggle("btn-light");
    },
  },
};
</script>

<style>
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
  background-color: rgba(86, 61, 124, 0.15);
  border: 1px solid rgba(86, 61, 124, 0.2);
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

.btn-group-xs > .btn,
.btn-xs {
  padding: 0.25rem 0.4rem;
  font-size: 0.875rem;
  line-height: 0.75;
  border-radius: 0.2rem;
}

[v-cloak] {
  display: none;
}

.material-icons.md-18 {
  font-size: 18px;
}

.material-icons.md-14 {
  font-size: 14px;
}

.material-icons.md-12 {
  font-size: 12px;
}

.w-10 {
  width: 10% !important;
}

.group-section {
  width: 150px;
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
  transition-property: color, background-color, border-color;
}
</style>
