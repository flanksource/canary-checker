<template>
    <div class="ml-4 mr-4" id="app">

        <auto-update-settings/>

        <h1>Canary Checker</h1>

        <hr>

        <error-panel :error="error"/>

        <table class="table table-sm table-fixed text-nowrap" id="checks" v-cloak>
            <thead>
            <th class="border-right">Type</th>
            <th class="border-right">NS/Name</th>
            <th class="border-right">Description</th>
            <th :key="server" class="border-right" v-for="(server, serverName) in serversByNames">{{ serverName }}</th>
            </thead>
            <template v-for="(typed, name) in groupedChecks">
                <tbody :key="type" class="border-bottom border-secondary" v-for="(mergedChecks, type) in typed">
                <tr :key="mergedDesc" v-for="(checkSet, mergedDesc, idx) in mergedChecks">
                    <td :rowspan="Object.keys(mergedChecks).length" class="align-middle border-right" v-if="idx === 0">
                        <img :src="'images/' + type + '.svg'" :title="type" height="20px">
                    </td>
                    <td :rowspan="Object.keys(mergedChecks).length" class="align-middle border-right w-10"
                        v-if="idx === 0">
                        <span :id="name" class="badge badge-secondary">{{ shortHand(name, nsLimit) }}</span>
                        <b-tooltip :target="name" triggers="hover" v-if="name.length > nsLimit" variant="secondary">
                            {{name}}
                        </b-tooltip>
                    </td>
                    <td class="align-middle w-25">
              <span :class="{'font-italic': mergedDesc.startsWith('multiple')}" :id="calcTooltipId(mergedDesc, name, type)"
                    class="float-left w-75 pr-5">
                {{ shortHand(mergedDesc, descLimit) }}
              </span>
                        <b-tooltip :disabled="mergedDesc.length <= descLimit"
                                   :target="calcTooltipId(mergedDesc, name, type)" triggers="hover" variant="secondary">
                            <div class="description">{{mergedDesc}}</div>
                        </b-tooltip>
                        <b-tooltip
                                :disabled="!mergedDesc.startsWith('multiple')"
                                :target="calcTooltipId(mergedDesc, name, type)"
                                triggers="hover"
                                variant="secondary">
                            <div :key="check.key" class="description" v-for="check in checkSet">{{check.description}}
                            </div>
                        </b-tooltip>
                        <button @click="triggerMerged(checkSet, $event)"
                                class="btn btn-info btn-xs float-right check-button" title="Trigger the check on every server">
                            <!--              <i class="material-icons md-12 align-middle">send</i>-->
                            <send-icon class="material-icons md-12 align-middle">send</send-icon>
                        </button>
                    </td>
                    <td :key="server" class="align-top border-right border-left" v-for="server in serversByNames">
                        <check-set-tds :check-set="checkSet" :server="server"></check-set-tds>
                    </td>
                </tr>
                </tbody>
            </template>
        </table>

        <div id="last-refreshed" v-cloak v-if="lastRefreshed">
            Last refreshed <span>{{ lastRefreshed }}</span>
        </div>
    </div>
</template>

<script>
    import Vuex from 'vuex'

    import store from './store'
    import AutoUpdateSettings from './components/AutoUpdateSettings.vue'
    import ErrorPanel from './components/ErrorPanel.vue'
    import CheckSetTds from './components/CheckSetTds.vue'

    export default {
        name: 'App',
        components: {
            AutoUpdateSettings,
            ErrorPanel,
            CheckSetTds
        },
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
            ...Vuex.mapState(['error', 'servers', 'lastRefreshed', 'checks', 'disableReload']),
            ...Vuex.mapGetters(['serversByNames', 'groupedChecks']),
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


    .btn-group-xs > .btn, .btn-xs {
        padding: .25rem .4rem;
        font-size: .875rem;
        line-height: .75;
        border-radius: .2rem;
    }

    [v-cloak] {
        display: none;
    }

    .material-icons.md-18 {
        font-size: 18px
    }

    .material-icons.md-14 {
        font-size: 14px
    }

    .material-icons.md-12 {
        font-size: 12px
    }

    .w-10 {
        width: 10% !important;
    }


    .group-section {
        width: 150px
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


</style>
