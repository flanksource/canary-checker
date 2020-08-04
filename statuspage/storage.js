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
      console.log('fetch')
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
    }
  }
})