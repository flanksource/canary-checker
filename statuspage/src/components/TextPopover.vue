<template>
  <b-popover
      :target="target"
      triggers="click"
      placement="top"
      custom-class="text-popover-wide"
      :delay="{ show: 150, hide: 0 }"
      boundary="window"
      boundary-padding="100"
      >
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
            <td v-if="!status.status" style="color: red" class="pre-formatted">{{status.message}}</td>
          </tr>
        </template>
      </table>
      <div class="left health" v-if="health != null" ><b>Avg latency:</b> {{health.latency}}<br/><b>Uptime: </b>{{health.uptime}}</div>
    </template>
  </b-popover>
</template>

<script>
import StatusStrip from "@/components/StatusStrip";
export default {
  name: "TextPopover",
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
.text-popover-wide {
  max-width: 100%; min-width: 32.5rem; max-height: 100%;
}
.pre-formatted {
  white-space: pre;
}
</style>