<template>
  <b-modal :id="target" :canCancel="['escape']" size="lg" ok-only>
    <template v-slot:title>
      <div class="description">{{ checkName }}</div>
    </template>
    <template>
      <table class="table table-sm table-fixed text-nowrap">
        <thead>
        <tr>
          <th class="border-right" >Duration</th>
          <th class="border-right" >Time</th>
          <th class="border-right" >Message</th>
        </tr>
        </thead>
        <template v-for="status in checkStatuses" >
          <tr :key="status.time">
            <td>{{status.duration / 1000}}s</td>
            <td>{{ timeago(status.time, false) }} Ago</td>
            <td v-if="status.status" style="color: green">{{status.message}}</td>
            <td v-if="!status.status" style="color: red" class="text-pre-formatted">{{status.message}}<br/>{{status.error}}</td>
          </tr>
        </template>
      </table>
      <div class="left health" v-if="health != null" ><b>Avg latency:</b> {{health.latency}}<br/><b>Uptime: </b>{{health.uptime}}</div>
    </template>
  </b-modal>
</template>

<script>
import StatusStrip from "@/components/StatusStrip";
export default {
  name: "TextModal",
  data() {
    return {
      elapsed: null,
      dateTime: null
    }
  },
  props: {
    target: {
      type: String,
      required: true
    },
    checkStatuses: {
      type: Array,
      required: true,
    },
    health: {
      type: Object,
      required: true
    },
    checkName: {
      type: String,
      required: true
    }
  },
  computed: {

  },
  methods: {
    timeago(time, s){
      return StatusStrip.methods.timeago().ago(time, s)
    }
  }
}
</script>
<style>
.text-pre-formatted {
  white-space: pre;
}
</style>