// A small textual display that serves as book-end
// for check info
<template>
    <div class="time-bookend" >{{ time }}</div>
</template>

<script>
    export default {
        name: "CheckTime",
        props: ['time'],
    }
</script>

<style scoped>
    div.time-bookend {
        display: inline-block;
        vertical-align: middle;
        font-size: xx-small;
        padding: 0.5em 1em;
    }
</style>