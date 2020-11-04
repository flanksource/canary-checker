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
            <tbody  class="border-bottom border-secondary" >
                <template v-for="(typed, name) in groupedChecks">
                    <!-- DELETE               <p :key="typed">{{name}}<br/> {{typed}} </p>-->
                    <template v-for="(mergedChecks, type) in typed">
                    <!-- DELETE                   <p> {{ type }}<br/>{{mergedChecks}}</p>-->
                      <template v-for="(checkSet, mergedDesc) in mergedChecks">
<!--                          <p :key="type+'-'+name+'-'+mergedDesc"> {{ type }}<br/>{{ name }}<br/>{{mergedDesc}}<br/></p>-->
<!--                          <pre :key="'json-'+type+'-'+name+'-'+mergedDesc">{{ JSON.stringify(checkSet, null, '\t') }}</pre>-->
                        <check :key="type+'-'+name+'-'+mergedDesc" :type="type" :name="name" :mergedDesc="mergedDesc"  :checkSet="checkSet" />
                      </template>
                    </template>
                </template>
            </tbody>
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
    import Check from "./components/Check";



    export default {
        name: 'App',
        components: {
            AutoUpdateSettings,
            ErrorPanel,
            Check,
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
