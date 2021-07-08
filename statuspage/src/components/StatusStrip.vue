<template>
    <div class="status-strip" >
        <div class="time-bookend" >{{latest}}</div>
      <template v-if="displayType !== 'Text'">
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
      </template>
      <template v-else-if="displayType==='Text'">
        <p v-if="check.checkStatuses[server][0].status"
           style="color: green"
           :id=check.key >{{firstLine(check.checkStatuses[server][0].message)}} </p>
        <p v-else-if="!check.checkStatuses[server][0].status"
           style="color: red;" class="pre-formatted"
           :id=check.key >{{firstLine(check.checkStatuses[server][0].message)}} </p>
        <text-popover 
            :check-statuses="check.checkStatuses[server]"
            :health="check.health[server]"
            :target="check.key" :check-name="check.name"
        />
      </template>
       <div class="time-bookend right" >{{earliest}}</div>
    </div>
</template>

<script>
    import BarPopover from "./BarPopover";
    import CheckPrometheus from "./CheckPrometheus";
    import TextPopover from "./TextPopover";

    export default {
        name: "StatusStrip",
        components: {
          TextPopover,
            BarPopover,
            CheckPrometheus,
        },
        props: {
            checks: {
                type: Array,
                required: true,
            },

            server: {
                type: String,
                required: true,
            },
            displayType: {
              type: String,
              required: false,
              default: "Bar"
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
            check() {
                return this.checks[0]
            },

            statii() {
                let statii = []

                for (const check of this.checks) {
                    if (check.checkStatuses[this.server] != null) {
                        statii.push(...check.checkStatuses[this.server])
                    }
                }
                return statii
            },
            fullWidth() {
                return (this.barWidth + this.barSpacing) * this.statii.length
            },
            barSet() {
                const scaling = [10,100,1000,10000,60000]
                let chosenScale = 10
                let barSet = []


                for (const statusData of this.statii) {
                    for (let scale = 0; scale < scaling.length - 1; scale++) {
                        if (statusData.duration >= scaling[scale]) {
                            chosenScale = scaling[scale + 1]
                        }
                    }
                }

                let i = 0
                for (const statusData of this.statii) {

                    let height = this.barMaxHeight // default to  a full bar
                    if (statusData.duration > 0) {
                        height = (statusData.duration/chosenScale)*this.barMaxHeight
                    }
                    height = Math.max(height, 2)
                    let bar = {
                        "key": statusData.key,
                        "width": this.barWidth,
                        "height": height,
                        "x": (this.barWidth + this.barSpacing) * i,
                        "y": this.barMaxHeight - height,
                        "color": statusData.status ? this.color : this.errorColor,
                        "checkStatus": statusData,
                        "description": this.check.description,
                        "message": statusData.message,
                        "health": this.check.health[this.server],
                        "duration": statusData.duration,
                        "time": statusData.time,
                        "checkType": this.check.type,
                        "endpoint": this.check.endpoint,
                        "canaryName": this.check.canaryName,
                    }
                    barSet.push(bar);
                    i++
                }
                return barSet;
            },
            latest() {
                var latestSoFar = null;
                for (const statusData of this.statii) {
                    const checkDate = new Date(statusData.time + " UTC");
                    if (latestSoFar === null || checkDate > latestSoFar) {
                        latestSoFar = checkDate
                    }
                }
                if (latestSoFar == null) {
                    return ""
                }
                if ((Date.now() - new Date(latestSoFar).getTime()) < 61 * 1000) {
                    return ""
                }

                return this.timeago().ago(latestSoFar, true)
            },
            earliest() {
                var earliestSoFar = null;
                for (const statusData of this.statii) {
                    const checkDate = new Date(statusData.time + " UTC");
                    if (earliestSoFar === null || checkDate < earliestSoFar) {
                        earliestSoFar = checkDate
                    }
                }
                if (earliestSoFar == null) {
                    return ""
                }

                  if ((Date.now() - new Date(earliestSoFar).getTime()) < 601 * 1000) {
                    return ""
                }

                return this.timeago().ago(earliestSoFar, true)
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
            },
            firstLine(message){
              return message.split("\n")[0]
            }
        },
    }
</script>

<style scoped>
    pre.nodata{
        display: inline-block;
        vertical-align: middle;
        font-size: xx-small;
        font-weight: bold;
        padding: 0.5em 1em;
        font-family: "Courier New", Courier, monospace;
        margin-bottom: 0;
    }
    div.time-bookend {
        display: inline-block;
        vertical-align: middle;
        font-size: xx-small;
        padding: 0.5em 1em;
    }
    div.time-bookend-right {
        float: right;
        vertical-align: middle;
        font-size: xx-small;
        padding: 0.5em 1em;
    }
    .pre-formatted {
      white-space: pre;
    }
</style>

