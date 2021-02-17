Vue.config.devtools = true
import Vue from 'vue'
import Vuex from 'vuex'
import Axios from 'axios'
import Lodash from 'lodash'

Vue.use(Vuex)


export default new Vuex.Store({
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
                    if (checkStatuses) {
                        for (let checkStatus of checkStatuses) {
                            checkStatus.key = window.btoa(check.key + server + checkStatus.time + check)
                        }
                    } else {
                        check.checkStatuses = []
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
        fetchData({ commit }) {
            commit('SET_LOADING', true)
            return Axios
                .get('/api/aggregate')
                .then((response) => {
                    commit('SET_CHECKS', response.data.checks)
                    commit('SET_SERVERS', response.data.servers)
                    commit('SET_LAST_REFRESHED', new Date())
                })
                .catch((err) => {
                    if (typeof err.response === 'undefined') {
                        commit('SET_ERROR', "Error: " + err.message);
                    }
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
        pauseAutoUpdate({ state, commit }) {
            commit('SET_DISABLE_RELOAD', true)
            clearInterval(state.reloadTimer)
            commit('SET_RELOAD_TIMER', null)
        },
        resumeAutoUpdate({ dispatch, commit }) {
            commit('SET_DISABLE_RELOAD', false)
            commit('SET_RELOAD_TIMER', setInterval(() => { dispatch('fetchData') }, 20000)) // 20 seconds
        },
        async triggerSingleCheck({ commit, dispatch }, { server, checkType, checkKey }) {
            try {
                await Axios
                    .post('/api/triggerCheck', { server, checkKey, checkType })
                dispatch('fetchData')
            } catch (err) {
                commit('SET_ERROR', "Trigger error: " + err.response.data)
            }
        },
        async triggerCheckOnAllServers({ state, commit/*, dispatch*/ }, check) {
            let results = []
            for (const server of state.servers) {
                if (check.checkStatuses[server]) {
                    results.push(Axios.post('/api/triggerCheck', { server, checkKey: check.key }))
                }
            }
            try {
                return Promise.all(results)
            } catch (err) {
                commit('SET_ERROR', "Trigger error: " + err.response.data)
            }
        },
        async triggerMergedChecks({ dispatch }, checks) {
            for (const check of checks) {
                await dispatch('triggerCheckOnAllServers', check)
            }
        }
    },
    getters: {
        orderedServers: state => {
            let servers = Lodash.uniqBy(Lodash.orderBy(
                state.servers.map(server => { return { label: server.split('@')[0], value: server } }), ["label"], ["asc"]),
                server => server.label
            )
            return servers
        },


        groupedChecks: state => {
            let group = {}

            for (const check of state.checks) {
                let groupBy = check.name + check.type + check.description
                if (group[check.namespace] == null) {
                    group[check.namespace] = {}
                }

                if (group[check.namespace][groupBy] == null) {
                    group[check.namespace][groupBy] = {
                        type: check.type,
                        namespace: check.namespace,
                        name: check.description ? check.description : check.name,
                        items: []
                    }
                }

                group[check.namespace][groupBy].items.push(check)
            }

            let ordered = []

            for (let ns in group) {
                let items = []
                for (let groupBy in group[ns]) {
                    items.push(group[ns][groupBy])
                }
                ordered.push({
                    namespace: ns,
                    items: Lodash.orderBy(items, ["name"], ["asc"]),
                })
            }
            return ordered
        }
    }
})
