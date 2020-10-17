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
                required: true,
                validator: prop => typeof prop === 'object' || prop === null
            },
        },
    }
</script>

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>

</style>
