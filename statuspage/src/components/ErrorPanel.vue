<template>
    <div class="alert alert-danger" role="alert" v-cloak v-if="error">
        {{ error }}
    </div>
</template>

<script>

    export default {
        name: 'ErrorPanel',
        props: {
            error: {
                type: Object,
                required: true
            },
        },
    }
</script>

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>

</style>
