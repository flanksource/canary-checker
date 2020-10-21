<template>
    <div class="status-strip" >
        <check-time :time="latest"/>
        <svg
                xmlns="http://www.w3.org/2000/svg"
                style="text-wrap: normal;"
                baseProfile="tiny"
                version="1.2"
                :width="fullWidth"
                :height="barMaxHeight">
            <g
                    v-for="(bar, index) in barSet"
                    :id="'bar-'+barSet[index].key"
                    :key="keyBar(barSet[index].key)">
                <!-- This rect is not for visual effect,-->
                <!-- but makes the following, actual    -->
                <!-- data bar easier to select when it  -->
                <!-- is narrow.                         -->
                <rect
                        :height="barMaxHeight" :width="barWidth"
                        :x="barSet[index].x"
                        :style=" {fill: 'white'}"/>
                <rect
                        :height="barSet[index].height" :width="barWidth"
                        :x="barSet[index].x" :y="barSet[index].y"
                        :style=" {fill: barSet[index].color}"/>
            </g>
        </svg>
        <check-time :time="earliest"/>
        <bar-popover
                v-for="bar in barSet"
                :key="keyPopover(bar.key)"
                :target="'bar-'+bar.key"
                :checkStatusKey="bar.key"
                :description="bar.description"
                :time="bar.time" :duration="bar.duration"
                :message="bar.message"
                :health="bar.health"/>
        <check-prometheus
                v-for="bar in barSet"
                :key="keyCheck(bar.key)"
                :checkType="bar.checkType"
                :check-key="bar.endpoint"
                :canary-name="bar.canaryName"
                :target-id="modalName(bar.key)"></check-prometheus>
    </div>
</template>

<script>
    import CheckTime from './CheckTime'
    import BarPopover from "./BarPopover";
    import CheckPrometheus from "./CheckPrometheus";

    export default {
        name: "StatusStrip",
        components: {
            CheckTime,
            BarPopover,
            CheckPrometheus,
        },
        props: {
            checkSet: {
                type: Array,
                required: true,
            },
            server: {
                type: String,
                required: true,
            },
            color: {
                type: String,
                default: 'green',
                required: false,
            },
            errorColor: {
                type: String,
                default: 'red',
                required: false,
            },
            barWidth: {
                type: Number,
                default: 200,
                required: false,
            },
            // When variances are small they are hard to
            // see: a zoominess of 0 does no zooming,
            //      a zoominess of 1 shows only the
            //      variances by chopping off the
            //      common minimum value.
            zoominess: {
                type: Number,
                default: 0,
                required: false,
            },
            barMaxHeight: {
                type: Number,
                default: 20,
                required: false,
            },
            barSpacing: {
                type: Number,
                default: 50,
                required: false,
            },
        },
        computed: {
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

                const sorted = this.$_.sortBy(statusesSet, function (statusData) {
                    return new Date(statusData.checkStatus.time + " UTC");
                }).reverse();

                const chunked = this.$_.chunk(sorted, serverRelatedCount * 2)

                return this.checkSet.length === 1 ? sorted : chunked[0]
            },
            fullWidth() {
                return (this.barWidth + this.barSpacing) * this.statusesSet.length
            },
            barSet() {
                let barSet = []
                //first find the minimum and maximum durations (skipping non-durations)
                //to be able to scale the data to fit in the allocated height
                let maxDuration = null
                let minDuration = null
                for (const statusData of this.statusesSet) {
                    if (!statusData.checkStatus.status) {
                        continue
                    }
                    if (maxDuration === null || statusData.checkStatus.duration > maxDuration) {
                        maxDuration = statusData.checkStatus.duration
                    }
                    if (minDuration === null || statusData.checkStatus.duration < minDuration) {
                        minDuration = statusData.checkStatus.duration
                    }
                }

                let i = 0
                for (const statusData of this.statusesSet) {
                    //scale the duration based on the minimum, maximum and zoominess
                    if (statusData.checkStatus.status) {
                        const offsetDuration = statusData.checkStatus.duration - minDuration * this.zoominess
                        const scaledDuration = offsetDuration / (maxDuration - minDuration * this.zoominess)
                        var normalizedDuration = scaledDuration * this.barMaxHeight
                        if (normalizedDuration < 0.5) {
                            // show at least a sliver for the minimum value
                            normalizedDuration = 0.5
                        }
                    } else {
                        // for an invalid sample we show a full bar
                        normalizedDuration = this.barMaxHeight
                    }

                    let bar = {
                        "key": statusData.checkStatus.key,
                        "width": this.barWidth,
                        "height": statusData.checkStatus.status ? normalizedDuration : this.barMaxHeight,
                        "x": (this.barWidth + this.barSpacing) * i,
                        "y": statusData.checkStatus.status ? (this.barMaxHeight - normalizedDuration) : 0,
                        "color": statusData.checkStatus.status ? this.color : this.errorColor,
                        "checkStatus": statusData.checkStatus,
                        "description": statusData.check.description,
                        "message": statusData.checkStatus.message,
                        "health": statusData.check.health[this.server],
                        "duration": statusData.checkStatus.duration,
                        "time": statusData.checkStatus.time,
                        "checkType": statusData.check.type,
                        "endpoint": statusData.check.endpoint,
                        "canaryName": statusData.check.canaryName,
                    }
                    barSet.push(bar);
                    i++
                }
                return barSet;
            },
            latest() {
                var latestSoFar = null;
                for (const statusData of this.statusesSet) {
                    const checkDate = new Date(statusData.checkStatus.time + " UTC");
                    if (latestSoFar === null || checkDate > latestSoFar) {
                        latestSoFar = checkDate
                    }
                }
                var ta = this.timeago();
                return ta.ago(latestSoFar, true)
            },
            earliest() {
                var earliestSoFar = null;
                for (const statusData of this.statusesSet) {
                    const checkDate = new Date(statusData.checkStatus.time + " UTC");
                    if (earliestSoFar === null || checkDate < earliestSoFar) {
                        earliestSoFar = checkDate
                    }
                }
                var ta = this.timeago();
                return ta.ago(earliestSoFar, true)
            },
        },
        methods: {
            modalName(key) {
                return "prometheus-modal-" + key
            },
            keyBar(key) {
                return "bar-" + key
            },
            keyPopover(key) {
                return "pop-" + key
            },
            keyCheck(key) {
                return "check-" + key
            },
            // Folowing timeago function is
            // from: https://github.com/digplan/time-ago/blob/master/timeago.js
            // License: MIT Copyright (c) 2015 Chris Borkert
            // https://github.com/digplan/time-ago/blob/master/license.txt
            timeago() {

                var o = {
                    second: 1000,
                    minute: 60 * 1000,
                    hour: 60 * 1000 * 60,
                    day: 24 * 60 * 1000 * 60,
                    week: 7 * 24 * 60 * 1000 * 60,
                    month: 30 * 24 * 60 * 1000 * 60,
                    year: 365 * 24 * 60 * 1000 * 60
                };
                var obj = {};

                obj.ago = function(nd, s) {
                    var r = Math.round,
                        dir = ' ago',
                        pl = function(v, n) {
                            return (s === undefined) ? n + ' ' + v + (n > 1 ? 's' : '') + dir : n + v.substring(0, 1)
                        },
                        ts = Date.now() - new Date(nd).getTime(),
                        ii;
                    if( ts < 0 )
                    {
                        ts *= -1;
                        dir = ' from now';
                    }
                    for (var i in o) {
                        if (r(ts) < o[i]) return pl(ii || 'm', r(ts / (o[ii] || 1)))
                        ii = i;
                    }
                    return pl(i, r(ts / o[i]));
                }

                obj.today = function() {
                    var now = new Date();
                    var Weekday = new Array("Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday");
                    var Month = new Array("January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December");
                    return Weekday[now.getDay()] + ", " + Month[now.getMonth()] + " " + now.getDate() + ", " + now.getFullYear();
                }

                obj.timefriendly = function(s) {
                    var t = s.match(/(\d).([a-z]*?)s?$/);
                    return t[1] * eval(o[t[2]]);
                }

                obj.mintoread = function(text, altcmt, wpm) {
                    var m = Math.round(text.split(' ').length / (wpm || 200));
                    return (m || '< 1') + (altcmt || ' min to read');
                }

                return obj;
            }
        },
    }
</script>

<style scoped>

</style>

