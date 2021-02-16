<!-- This component encapsulates a pop-over that is displayed -->
<!-- when hovering of a check infographic                     -->
<template>
    <b-popover
            :target="target"
            triggers="hover"
            placement="top"
            :delay="{ show: 150, hide: 0 }"
            @show="onShow">
        <template v-slot:title>
            <div class="description">{{description}}</div><div>{{elapsed}}</div>
        </template>
        <template v-slot:default>
            <div>{{message}}</div>
            <div class="duration">Duration: {{duration / 1000}}s <br/>{{dateTime}}</div>
            <hr/>
            <div class="left health" v-if="health != null" >Avg latency: {{health.latency}}<br/>Uptime: {{health.uptime}}</div>
            <button class="btn btn-info btn-xs float-right check-button mb-2" @click="triggerCheck" title="Trigger the check on particular server">
                <sync-icon class="material-icons md-14 align-middle" />
            </button>
            <button class="btn btn-warning btn-xs float-right check-button mb-2 prometheus-graph" v-b-modal="modalName(checkStatusKey)" title="Open Prometheus graph">
                <chart-bar-icon class="material-icons md-14 align-middle" />
            </button>
        </template>
    </b-popover>
</template>

<script>
    import timeago from 'timeago-simple';
    import date from 'date-and-time';
    import moment from 'moment';



    export default {
        name: "BarPopover",
        data() {
            return {
                elapsed: null,
                dateTime: null
            }
        },
        props: {
            target: {
                type: String,
                required: true
            },
            checkStatusKey: {
                type: String,
                required: true
            },
            description: {
                type: String,
                required: true
            },
            message: {
                type: String,
                required: true
            },
            time: {
                type: String,
                required: true
            },
            duration: {
                type: Number,
                required: true
            },
            health: {
                type: Object,
                required: false
            },
        },
        methods: {
            onShow() {
                const dateTime = new Date(this.time + " UTC");
                this.elapsed = timeago.simple(date.format(dateTime, 'YYYY-MM-DD HH:mm:ss', false), 'en_US')
                this.dateTime = this.moment(dateTime).format()
            },
            triggerCheck() {
                this.$root.$emit('bv::hide::popover')
                // this.$refs.popover.$emit('close')
                this.$emit('triggerCheck')
            },
            modalName(key) {
                return "prometheus-modal-" + key
            },
            // make moment() accessible in component
            // see https://stackoverflow.com/a/34310642
            moment: function () {
                return moment();
            },
        }
    }
</script>

<style scoped>

</style>
