<template>
    <tr class="border-bottom-0 border-top-0">
        <td class=" w-5 border-top-0 border-bottom-0"></td>
        <td class="align-middle  border-top-0 border-bottom-0">
            <img :src="'images/' + type + '.svg'" :title="type" height="20px">
        </td>
        <td class="align-middle border-left border-right">
            <span :id="name">{{ shortHand(name, nsLimit) }}</span>
            <b-tooltip :target="name" triggers="hover" v-if="name.length > nsLimit" variant="secondary">
                {{name}}
            </b-tooltip>
        </td>
        <td class="align-middle">
                              <span :class="{'font-italic': mergedDesc.startsWith('multiple')}"
                                    :id="calcTooltipId(mergedDesc, name, type)"
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
                    class="btn btn-info btn-xs float-right check-button"
                    title="Trigger the check on every server">
                <send-icon class="material-icons md-12 align-middle">send</send-icon>
            </button>
        </td>
        <td :key="server" class="align-top border-right border-left" v-for="server in serversByNames">
            <div>
                <status-strip :bar-spacing="5" :bar-width="20"
                              :barMaxHeight="20" :check-set="checkSet"
                              :server="server" :zoominess="0.85" color="#28a745"
                              error-color="#dc3545"/>
            </div>
        </td>
    </tr>
</template>

<script>
    import StatusStrip from './StatusStrip.vue'
    import Vuex from "vuex";

    export default {
        name: "Check",
        components: {
            StatusStrip,
        },
        props: {
            name: {
                type: String,
                required: true
            },
            mergedDesc: {
                type: String,
                required: true
            },
            checkSet: {
                type: Array,
                required: true
            },
            type: {
                type: String,
                required: true
            }
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

<style scoped>
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

    .btn-group-xs > .btn, .btn-xs {
        padding: .25rem .4rem;
        font-size: .875rem;
        line-height: .75;
        border-radius: .2rem;
    }

    [v-cloak] {
        display: none;
    }


    .material-icons.md-12 {
        font-size: 12px
    }

    .check-button {
        transition: all 0.4s linear;
        transition-property: color, background-color, border-color
    }

</style>