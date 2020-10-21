<template>
    <button class="btn btn-danger float-right" type="button" v-cloak v-if="disableReload"
            v-on:click="resumeAutoUpdate">
        <play-icon class="material-icons md-18 align-middle"/>
        <span class="align-middle">Resume auto update</span>
    </button>
    <button class="btn btn-primary float-right" type="button" v-else v-on:click="pauseAutoUpdate">
        <pause-icon class="material-icons md-18 align-middle"/>
        <span class="align-middle">Pause auto update</span>
    </button>
</template>

<script>
    import store from "../store/index.js";
    import Vuex from "vuex";

    export default {
        name: 'AutoUpdateSettings',
        store: store,
        computed: {
            ...Vuex.mapState(['error', 'servers', 'lastRefreshed', 'checks', 'disableReload']),
            ...Vuex.mapGetters(['serversByNames', 'groupedChecks']),
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

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>
    /*Fixes ugly bottom-aligned pause icon*/
    .pause-icon .material-design-icon__svg {
        bottom: unset;
    }
    /*Fixes ugly bottom-aligned play icon*/
    .play-icon .material-design-icon__svg {
        bottom: unset;
    }
</style>
