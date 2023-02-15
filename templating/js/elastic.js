elastic = {
  getIndexMetrics: function (results) {
    components = []
    for (i in results) {
      index = results[i].Object
      name = 0 //TODO
      documents = index.primaries.docs.count
      size = index.primaries.store.size_in_bytes
      healthy_indices = 0 //TODO
      unhealthy_indices = index.primaries.indexing.index_failed
      components.push({
        name: name,
        properties: [
          {
            name: "documents",
            value: documents
          },
          {
            name: "size",
            value: size
          },
          {
            name: "healthy_indices",
            value: healthy_indices
          },
          {
            name: "unhealthy_indices",
            value: unhealthy_indices
          }
        ]
      })
    }
    return components
  },
  getNodeMetrics: function (results) {
    components = []
    for (i in results) {
      node = results[i].Object
      version = node.versions
      disk_space_used = node.os.fs.available
      disk_space_free = node.os.fs.total
      components.push({
        name: name,
        properties: [
          {
            name: "version",
            value: version
          },
          {
            name: "disk_space_used",
            value: disk_space_used
          },
          {
            name: "disk_space_free",
            value: disk_space_free
          }
        ]
      })
    }
    return components
  }
}
