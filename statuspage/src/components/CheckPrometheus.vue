<template>
    <b-modal
            :id="targetId"
            @show="onShow"
            custom-class="prometheus-popover"
            size='lg'>
        <template v-slot:modal-title>
            <div class="description">Prometheus Graph <span class="badge badge-danger">{{ checkType }}</span> <span
                    class="badge badge-secondary">{{ checkKey }}</span></div>
        </template>
        <div aria-label="Timeframe" class="btn-group" role="group">
            <button :class="btnClass(ts.value)"
                    type="button"
                    v-for="ts in timeSelector"
                    :key="keyButton(targetId,ts)"
                    v-on:click="setSelector(ts.value)">{{ ts.name }}
            </button>
        </div>

        <line-chart :canary-name="canaryName" :check-key="checkKey" :check-type="checkType" :key="keySuccess(currentSelector)"
                    :styles="chartStyle" :time-selector="currentSelector" field="success"
                    name="Success"></line-chart>
        <hr/>

        <line-chart :canary-name="canaryName" :check-key="checkKey" :check-type="checkType" :key="keyFailed(currentSelector)" :styles="chartStyle"
                    :time-selector="currentSelector" field="failed" name="Failed"></line-chart>
        <hr/>

        <line-chart :canary-name="canaryName" :check-key="checkKey" :check-type="checkType" :key="keyLatency(currentSelector)"
                    :styles="chartStyle" :time-selector="currentSelector" field="latency"
                    name="Latency"></line-chart>
        <hr/>

    </b-modal>
</template>

<script>
    import LineChart from "./LineChart";

    export default {
        name: 'CheckPrometheus',
        components: {
            LineChart,
        },
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
            keyButton(targetId,ts) {
                console.log(targetId+'-'+ts.name)
                return targetId+'-'+ts.name
            },
            keySuccess(cs) {
                return "success-" + cs
            },
            keyFailed(cs) {
                return "failed-" + cs
            },
            keyLatency(cs) {
                return "latency-" + cs
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

</style>
