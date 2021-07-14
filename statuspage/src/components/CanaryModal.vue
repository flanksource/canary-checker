<!-- This component encapsulates a modal that is displayed -->
<!-- when clicking on canary                               -->
<template>
  <!--  Component modal doc: : https://bootstrap-vue.org/docs/components/modal-->
  <!--  Here we use dynamic id for the modal since it is being called from a loop.-->
  <!--  If ID is not distinguished then a click on any row will open modal for every value-->
  <b-modal :id="`modal-canary${name}${namespace}`" ok-only>
    <template #modal-title>
      <img :src="`images/${checkType}.svg`" v-bind:style="{ height: '1.875rem' }" alt=""> {{name}}
    </template>
    <ul class="list-unstyled">
      <li><b>Name:</b> {{ handleMissingValue(name) }}</li>
      <li><b>Namespace:</b> {{ handleMissingValue(namespace) }}</li>
      <li><b>Description:</b> {{ handleMissingValue(description) }}</li>
      <li><b>Interval:</b> {{  handleMissingValue(interval) }} Seconds</li>
      <li><b>Owner:</b> {{ handleMissingValue(owner) }}</li>
      <li><b>Severity:</b> {{ handleMissingValue(severity) }}</li>
    </ul>
  </b-modal>
</template>


<script>
export default {
  name: "CanaryModal",
  props: {
    interval: {
      type: Number,
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
    handleMissingValue(prop){
      if (prop != null) {
        if(typeof prop === 'string' && prop.length === 0) {
          return "-"
        }
        return prop
      }
      return "-"
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
