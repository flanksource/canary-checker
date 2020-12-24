<template>
    <div class="ml-4 mr-4" id="app">

        <auto-update-settings/>

        <h1>Canary Checker</h1>

        <hr>

        <error-panel :error="error"/>

        <table class="table table-fixed table-sm text-nowrap w-auto" id="checks" v-cloak>
            <thead class="border-0">
            <tr class="border-0">
                <th class="min border-0 ">NS</th>
                <th class="min  border-0">Type</th>
                <th class="min border-0">Name</th>
                <th class="min border-0 ">Description</th>
                <th :key="server" class="min  border-0" v-for="(server, serverName) in serversByNames">{{ serverName }}</th>
                <!--  this invisible borderless "spacer" column fills the rest of the widths-->
                <td class="border-0"></td>
            </tr>
            </thead>

            <template v-for="(typed, name) in groupedChecks">
                <tbody :key="name" class="border-bottom border-top-0">
                    <!-- Namespace Header for this grouped check-->
                    <tr :key="name" class="pt-6 namespace border-top-0 border-bottom-0">
                        <td class="border-0 namespace" colspan="4">
                            <span class="badge badge-secondary">{{name.split('/')[0]}}</span>
                        </td>
                    </tr>
                    <template v-for="(mergedChecks, type) in typed">
                        <template v-for="(checkSet, mergedDesc) in mergedChecks">
                            <check :checkSet="checkSet" :key="type+'-'+name+'-'+mergedDesc" :mergedDesc="mergedDesc"
                                   :name="name.split('/')[1]" :type="type"/>
                        </template>
                    </template>
                </tbody>
            </template>

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
        margin-top: 0;
    }

    .popover-body > hr {
        margin: 0.4rem 0;
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

    [v-cloak] {
        display: none;
    }

    /*These 'min'-classed columns should take minimal width based on content*/
    th.min {
        width: 1%;
        white-space: nowrap;
    }

    /*The 'namespace'-classed rows should have a bit of extra vertical seperation space*/
    tr.namespace {
        height: 2.5rem;
    }

    td.namespace {
        vertical-align: bottom;
    }

</style>
