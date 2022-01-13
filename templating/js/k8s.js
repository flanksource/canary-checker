
k8s = {
  conditions: {
    getMessage: function (v) {
      message = ""
      if (v.status == null) {
        return "No status found"
      }
      status = v.status
      if (status.conditions == null) {
        return "no conditions found"
      }
      status.conditions.forEach(function (state) {
        if (state.status != "True") {
          message += state.type
          message += " "
        }
      })
      return message.trim()
    },
    getError: function (v) {
      active = []
      if (v.status == null) {
        return "No status found"
      }
      status = v.status
      if (status.conditions == null) {
        return "no conditions found"
      }
      status.conditions.forEach(function (state) {
        if (state.status == "False") {
          active.push(state)
        }
      })
      active.sort(function (a, b) { a.lastTransitionTime > b.lastTransitionTime && 1 || -1 })
      errorMessage = ""
      active.forEach(function (state) {
        if (errorMessage != "") {
          errorMessage += ', '
        }
        errorMessage += state.lastTransitionTime + ': ' + state.type + ' is ' + state.reason
        if (state.message != null) {
          errorMessage += ' with ' + state.message
        }
      })
      return errorMessage
    },
    isReady: function (v) {
      if (v.status == null) {
        return false
      }
      status = v.status
      if (status.conditions == null) {
        return false
      }
      ready = true
      status.conditions.forEach(function (state) {
        if (state.type == "Ready") {
          if (state.status != "True") {
            ready = false
          }
        }
      })
      return ready
    },
  },
  getAlertName: function (v) {
    name = v.alertname
    if (startsWith(v.alertname, "KubeDeployment")) {
      return name + "/" + v.deployment
    }
    if (startsWith(v.alertname, "KubePod") || startsWith(v.alertname, "ExcessivePod")) {
      return name + "/" + v.pod
    }
    if (startsWith(v.alertname, "KubeDaemonSet")) {
      return name + "/" + v.daemonset
    }
    if (v.alertname == "CertManagerInvalidCertificate") {
      return name + "/" + v.name
    }
    if (startsWith(v.alertname, "KubeStatefulSet")) {
      return name + "/" + v.statefulset
    }
    if (startsWith(v.alertname, "Node") || startsWith(v.alertname, "KubeNode")) {
      return name + "/" + v.node
    }
  },
  getAlertLabels: function (v) {
    function ignoreLabel(k) {
      return k == "severity" || k == "job" || k == "alertname" || k == "alertstate" || k == "__name__" || k == "value" || k == "namespace"
    }
    function parseLabels(v) {
      results = {}
      v.namespace = v.namespace || v.exported_namespace
      v.instance = v.exported_instance
      delete (v.exported_namespace)
      delete (v.exported_instance)
      for (k in v) {
        newKey = k.replace("label_", "")
        newKey = newKey.replace("apps_kubernetes_io_", "apps/kubernetes.io/")
        results[newKey] = v[k]
        delete v[k]
      }
      return results
    }
    v = parseLabels(v)
    if (v.alertname == "CertManagerInvalidCertificate") {
      delete (v.condition)
      delete (v.container)
      delete (v.endpoint)
      delete (v.instance)
      delete (v.service)
      delete (v.pod)
    }
    return v
  },
  getAlerts: function (results) {
    function ignoreLabel(k) {
      return k == "severity" || k == "job" || k == "alertname" || k == "alertstate" || k == "__name__" || k == "value" || k == "namespace"
    }
    function getLabels(v) {
      s = ""
      for (k in v) {
        if (ignoreLabel(k)) {
          continue
        }
        if (s != "") {
          s += " "
        }
        s += k + "=" + v[k]
      }
      return s
    }
    function getLabelMap(v) {
      out = {}
      for (k in v) {
        if (ignoreLabel(k)) {
          continue
        }
        out[k] = v[k] + ""
      }
      return out
    }
    var out = _.map(results, function (v) {
      v = k8s.getAlertLabels(v)
      return {
        pass: v.severity == "none",
        namespace: v.namespace,
        labels: getLabelMap(v),
        message: getLabels(v),
        name: k8s.getAlertName(v)
      }
    })
    JSON.stringify(out)
  },
  getNodeMetrics: function (results) {
    components = []
    for (i in results) {
      node = results[i].Object
      components.push({
        name: node.metadata.name,
        properties: [
          {
            name: "cpu",
            value: fromMillicores(node.usage.cpu)
          },
          {
            name: "memory",
            value: fromSI(node.usage.memory)
          }
        ]
      })
    }
    return components
  },
  getNodeTopology: function (results) {
    var nodes = []
    for (i in results) {
      node = results[i].Object
      _node = {
        name: node.metadata.name,
        properties: [
          {
            name: "cpu",
            min: 0,
            unit: "millicores",

            max: fromMillicores(node.status.allocatable.cpu)
          },
          {
            name: "memory",
            unit: "bytes",
            max: fromSI(node.status.allocatable.memory)
          },
          {
            name: "ephemeral-storage",
            unit: "bytes",

            max: fromSI(node.status.allocatable["ephemeral-storage"])
          },
          {
            name: "instance-type",
            text: node.metadata.labels["beta.kubernetes.io/instance-type"]
          },
          {
            name: "zone",
            text: node.metadata.labels["topology.kubernetes.io/zone"]
          },
          {
            name: "ami",
            text: node.metadata.labels["eks.amazonaws.com/nodegroup-image"]
          }
        ]
      }
      internalIP = _.find(node.status.addresses, function (a) { a.type == "InternalIP" })
      if (internalIP != null) {
        _node.properties.push({
          name: "ip",
          text: internalIP.address
        })
      }
      externalIP = _.find(node.status.addresses, function (a) { a.type == "ExternalIP" })
      if (externalIP != null) {
        _node.properties.push({
          name: "externalIp",
          text: externalIP.address
        })
      }
      for (k in node.status.nodeInfo) {
        if (k == "bootID" || k == "machineID" || k == "systemUUID") {
          continue
        }
        v = node.status.nodeInfo[k]
        _node.properties.push({
          name: k,
          text: v
        })
      }

      if (k8s.conditions.isReady(node)) {
        _node.status = "healthy"
      } else {
        _node.status = "unhealthy"
        _node.statusReason = k8s.conditions.getMessage(node)
      }

      nodes.push(_node)
    }
    return nodes
  }
}
