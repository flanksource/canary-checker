Vue.config.devtools = true
Vue.use(Vuex)

const store = new Vuex.Store({
  state: {
    error: null,
    loading: true,
    checks: [],
    servers: [],
    lastRefreshed: null,
    disableReload: true,
    reloadTimer: null,
  },
  mutations: {
    SET_SERVERS(state, servers) {
      state.servers = servers
    },
    SET_CHECKS(state, checks) {
      for (let check of checks) {
        for (let [server, checkStatuses] of Object.entries(check.checkStatuses)) {
          for (let checkStatus of checkStatuses) {
            checkStatus.key = window.btoa(check.key + server + checkStatus.time)
          }
        }
      }
      state.checks = checks
    },
    SET_LOADING(state, loading) {
      state.loading = loading
    },
    SET_ERROR(state, error) {
      state.error = error
    },
    SET_LAST_REFRESHED(state, lastRefreshed) {
      state.lastRefreshed = lastRefreshed
    },
    SET_DISABLE_RELOAD(state, disableReload) {
      state.disableReload = disableReload
    },
    SET_RELOAD_TIMER(state, reloadTimer) {
      state.reloadTimer = reloadTimer
    }
  },
  actions: {
    fetchData({commit}) {
      commit('SET_LOADING', true)
      return axios
        .get('/api/aggregate')
        .then((response) => {
          commit('SET_CHECKS', response.data.checks)
          commit('SET_SERVERS', response.data.servers)
          commit('SET_LAST_REFRESHED', new Date())
        })
        .catch((err) => {
          if (err.response.status === 0) {
            commit('SET_ERROR', "Error loading data from server: failed to connect to server")
          } else {
            commit('SET_ERROR', "Error loading data from server: " + err.response.data)
          }
        })
        .finally(() => {
          commit('SET_LOADING', false)
        })
    },
    pauseAutoUpdate ({state, commit}) {
      commit('SET_DISABLE_RELOAD', true)
      clearInterval(state.reloadTimer)
      commit('SET_RELOAD_TIMER', null)
    },
    resumeAutoUpdate({dispatch, commit}) {
      commit('SET_DISABLE_RELOAD', false)
      commit('SET_RELOAD_TIMER', setInterval(() => { dispatch('fetchData') }, 20000)) // 20 seconds
    },
    triggerSingleCheck({commit, dispatch}, {server, check}) {
      return axios
        .post('/api/triggerCheck', { server, checkKey: check.key })
        .then(() => {
          dispatch('fetchData')
        })
        .catch((err) => {
          commit('SET_ERROR', "Trigger error: " + err.response.data)
        })
    },
    triggerCheckOnAllServers({state, commit, dispatch}, {check}) {
      let results = []
      for (const server of state.servers) {
        if (check.checkStatuses[server]) {
          results.push(axios.post('/api/triggerCheck', { server, checkKey: check.key }))
        }
      }
      return Promise.all(results)
        .catch((err) => {
          commit('SET_ERROR', "Trigger error: " + err.response.data)
        })
        .finally(() => {
          dispatch('fetchData')
        })
    },
  },
  getters: {
    serversByNames: state => {
      return _.fromPairs(state.servers.map(server => [server.split('@')[0], server]))
    },
    groupedChecks: state => {
      const byName = _.groupBy(state.checks, 'name')
      for (const [name, checks] of Object.entries(byName)) {
        byName[name] = _.groupBy(checks, 'type')
      }
      return byName
    }
  }
})