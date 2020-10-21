<!--<template>-->
<!--    -->
<!--</template>-->

<script>
    import {mixins, Line} from 'vue-chartjs'
    import axios from 'axios'
    import moment from 'moment';

    export default {
        name: "LineChart",
        extends: Line,
        mixins: [mixins],
        async created() {
            await this.fetchData(this.timeSelector)
        },
        data() {
            return {
                seriesLabels: [],
                seriesValues: [],
                datacollection: null,
                options: {
                    maintainAspectRatio: false,
                    scales: {
                        yAxes: [{
                            id: 'Value',
                            type: 'linear',
                            offset: true,
                            ticks: {
                                min: 0,
                            },
                        }],
                        xAxes: [{
                            id: 'Time',
                            ticks: {
                                maxRotation: 0,
                                autoSkipPadding: 30,
                            },
                        }]
                    },
                },
            }
        },
        props: {
            //'options'
            name: {
                type: String,
                required: true,
            },
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
            field: {
                type: String,
                required: true,
            },
            timeSelector: {
                type: Number,
                required: true
            }
        },
        methods: {
            fillData () {
                this.datacollection = {
                    labels: this.seriesLabels,
                    datasets: [
                        {
                            borderColor: "#dc3545",
                            fill: false,
                            cubicInterpolationMode: 'monotone',
                            label: this.name,
                            backgroundColor: '#f87979',
                            data: this.seriesData,
                        }
                    ]
                }
            },
            fetchData() {
                axios
                    .post('/api/prometheus/graph', { checkType: this.checkType, canaryName: this.canaryName, checkKey: this.checkKey, timeframe: this.timeSelector})
                    .then((response) => {
                        var data = response.data[this.field]
                        this.seriesLabels = data.map(x => this.formatLabel(x.time))
                        this.seriesData = data.map(x => this.formatValue(x.value))
                        this.fillData()
                        this.renderChart(this.datacollection, this.options)
                    })
                    .catch((err) => {
                        if (err.response === undefined) {
                            console.log("Error: " + err)
                        } else if (err.response.status === 0) {
                            console.log("Error loading data from server: failed to connect to sercer")
                        } else {
                            console.log("Error loading data from server: failed: " + err.response.data)
                        }
                    })
            },
            formatLabel(label) {
                if (this.currentSelector > (3600 * 24)) {
                    return this.moment(label * 1000).format("D/M HH:mm")
                }
                return this.moment(label * 1000).format("HH:mm")
            },
            formatValue(value) {
                return Math.round(parseFloat(value, 10))
            },
            // make moment() accessible in component
            // see https://stackoverflow.com/a/34310642
            moment: function () {
                return moment();
            },
        }
    }
</script>

<!--<style scoped>-->

<!--</style>-->