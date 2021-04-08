<!-- This component encapsulates a modal that is displayed -->
<!-- when clicking on canary                               -->
<template>
<!--  Here we use dynamic id for the modal since it is being called from a loop.-->
<!--  If ID is not distinguished then a click on any row will open modal for every value-->
  <b-modal :id="`modal-canary${name}${namespace}`" ok-only>
    <template #modal-title>
      <img :src="`images/${checkType}.svg`" v-bind:style="{ height: '1.875rem' }" alt=""> {{name}}
    </template>
    <ul class="list-unstyled">
      <li><b>Name:</b> {{name}}</li>
      <li><b>Namespace:</b> {{namespace}}</li>
      <li><b>Description:</b> {{description}}</li>
      <li><b>Interval:</b> {{interval}} Seconds</li>
      <li><b>Owner:</b> {{getProp(owner)}}</li>
      <li><b>Severity:</b> {{getProp(severity)}}</li>
    </ul>
  </b-modal>

</template>
<script>



export default {
  name: "CanaryModal",
  props: {
    interval: {
      type: String,
      required: true
    },
    owner: {
      type: String,
      required: true
    },
    severity: {
      type: String,
      required: true
    },
    name: {
      type: String,
      required: true
    },
    namespace: {
      type: String,
      required: true,
    },
    description: {
      type: String,
      required: true,
    },
    checkType: {
      type: String,
      required: true,
    },
  },
  methods: {
    getProp(prop){
      if (prop.length === 0) {
        return "-"
      }
      return prop
    }
  }
}
</script>

<style scoped>
.list-unstyled {
  list-style: none;
  margin-left: 0;
  padding-left: 0;
}
</style>
