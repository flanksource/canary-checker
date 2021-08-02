<!-- This component encapsulates the table rows that is displayed -->
<!-- when clicking on canary                               -->
<template>
  <tr :key="checkKey">
    <!--  Component modal doc: : https://bootstrap-vue.org/docs/components/modal-->
    <td v-b-modal="`modal-canary-${checkKey}`">
      <img :src="`images/${checkType}.svg`" :title="checkType " v-bind:style="{ height: '1.25rem' }" :alt="`${checkType} logo`" >  {{ shortDescription }}
      <canary-modal :interval="interval"
                    :owner="owner"
                    :severity="severity"
                    :check-type="checkType"
                    :description="description"
                    :name="name"
                    :namespace="namespace"
                    :check-key="checkKey"
      />
    </td>
    <td v-for="server in orderedServers" :key="server.value" class="align-top border-right border-left">
      <div>
        <status-strip :checks="items" :server="server.value" :display-type="displayType"
                      color="#28a745" error-color="#dc3545"
                      :bar-width="20" :bar-spacing="5" :barMaxHeight="20"
                      :zoominess="0.85"/>
      </div>
    </td>
  </tr>
</template>
<script>
import CanaryModal from "@/components/CanaryModal";
import StatusStrip from "@/components/StatusStrip";
import Vuex from "vuex";
export default {
  name: "CanaryStandardServerRow",
  components: {StatusStrip, CanaryModal},
  computed: {
    ...Vuex.mapState([
      "servers",
    ]),
    ...Vuex.mapGetters(["orderedServers"]),
  },
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
    shortDescription: {
      type: String,
      required: true
    },
    items: {
      type: Array,
      required: true
    },
    checkKey: {
      type: String,
      required: true
    },
    displayType: {
      type: String,
      required: true
    },
  },
}
</script>