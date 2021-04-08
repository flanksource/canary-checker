<!-- This component encapsulates a modal that is displayed -->
<!-- when clicking on canary                               -->
<template>
  <b-modal :id="'modal-canary'+name+namespace" ok-only>
    <template #modal-title>
      <img :src="'images/' + checkType + '.svg'" height="30px"> {{name}}
    </template>
    <template v-slot:default>
      <div class="name"><b>Name:</b> {{name}}</div>
      <div class="namespace"><b>Namespace:</b> {{namespace}}</div>
      <div class="description"><b>Description:</b> {{description}}</div>
      <div class="interval"><b>Interval:</b> {{interval}} Seconds</div>
      <div class="owner" v-if="owner.length !== 0"><b>Owner:</b> {{owner}}</div>
      <div class="severity" v-if="severity.length !== 0"><b>Severity:</b> {{severity}}</div>
    </template>
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
}
</script>

<style scoped>

</style>
