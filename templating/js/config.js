config = {
  getConfigItem: function(configItemID) {
    var [type, name] = configItemID.split('/')
    configObject = findConfigItem(type, name)
    return configObject
  }
}
