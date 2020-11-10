// A small textual display that serves as book-end
// for check info
<template>
    <div class="time-bookend" ><pre>{{ time }}</pre></div>
</template>

<script>
    export default {
        name: "CheckTime",
        props: ['time'],
    }
</script>

<style scoped>
    .time-bookend {
        display: inline-block;
        vertical-align: middle;
        font-size: xx-small;
        font-weight: bold;
        padding: 0.5em 1em;
        font-family: "Courier New", Courier, monospace;
        margin-bottom: 0;
    }
</style>