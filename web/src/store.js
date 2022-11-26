import Vue from 'vue';
import Vuex from 'vuex';

import APIClient from './apiClient';

Vue.use(Vuex);

const store = new Vuex.Store({
    state: {
        searchResponse: null,
        searchRunning: false,
        searchPage: null,
        searchQuery: null,

        streamData: null,
        streamLoading: false,
        streamIndex: null,

        status: null,
        pcaps: null,

        tags: null,
        tagAddStatus: null,
        tagDelStatus: null,

        graphData: null,

        markTagNewStatus: null,
        markTagUpdateStatus: null,

        streams: {
            query: null,
            page: null,
            running: false,
            error: null,
            result: null,
        },
        stream: {
            id: null,
            running: false,
            error: null,
            stream: null,
        },
        graph: {
            type: null,
            delta: null,
            aspects: null,
            tags: null,
            query: null,
            running: false,
            error: null,
            graph: null,
        },
    },
    mutations: {
        setStreams(state, obj) {
            state.streams = obj;
        },
        setStream(state, obj) {
            state.stream = obj;
        },
        setGraph(state, obj) {
            state.graph = obj;
        },
        updateMark(state, { name, streams, value }) {
            if (state.stream.stream != null && (streams == undefined || streams.includes(state.stream.stream.Stream.ID))) {
                const s = state.stream.stream;
                const current = s.Tags.includes(name);
                if (value && !current) {
                    s.Tags.push(name);
                } else if (current && !value) {
                    s.Tags = s.Tags.filter((t) => t != name)
                }
            }
            if (state.streams.result != null) {
                for (const s of state.streams.result.Results) {
                    if (streams != undefined && !streams.includes(s.Stream.ID)) continue;
                    const current = s.Tags.includes(name);
                    if (value && !current) {
                        s.Tags.push(name);
                    } else if (current && !value) {
                        s.Tags = s.Tags.filter((t) => t != name)
                    }
                }
            }
        },
        searchStarted(state, obj) {
            state.streamData = null;
            state.streamIndex = null;
            state.streamLoading = false;

            state.searchResponse = null;
            state.searchPage = obj.page;
            state.searchQuery = obj.query
            state.searchRunning = true;
        },
        resetSearchResponse(state, searchResponse) {
            state.searchResponse = searchResponse;
            state.searchRunning = false;
            if (searchResponse && searchResponse.Debug) {
                searchResponse.Debug.map((s) => { console.log(JSON.parse(JSON.stringify(s))); })
            }
        },
        resetStreamIndex(state, streamIndex) {
            state.streamLoading = true;
            state.streamData = null;
            state.streamIndex = streamIndex;
        },
        resetStreamData(state, stream) {
            state.streamData = stream;
            state.streamLoading = false;
        },
        resetStatus(state, status) {
            state.status = status;
        },
        resetTags(state, tags) {
            state.tags = tags;
        },
        resetPcaps(state, pcaps) {
            state.pcaps = pcaps;
        },
        resetTagAddStatus(state, status) {
            state.tagAddStatus = status
        },
        resetTagDelStatus(state, status) {
            state.tagDelStatus = status
        },
        resetGraphData(state, data) {
            state.graphData = data;
        },
        resetMarkTagNewStatus(state, status) {
            state.markTagNewStatus = status
        },
        resetMarkTagUpdateStatus(state, status) {
            state.markTagUpdateStatus = status
        },
    },
    getters: {
        prevSearchPage(state) {
            if (state.searchPage == null || state.searchPage <= 0)
                return null;
            return state.searchPage - 1;
        },
        nextSearchPage(state) {
            if (state.searchPage == null || state.searchResponse == null || !state.searchResponse.MoreResults)
                return null;
            return state.searchPage + 1;
        },
        prevStreamIndex(state) {
            if (state.streamIndex == null || state.streamIndex <= 0)
                return null;
            return state.streamIndex - 1;
        },
        nextStreamIndex(state) {
            if (state.streamIndex == null || state.searchResponse == null || state.searchResponse.Results == null || state.streamIndex + 1 >= state.searchResponse.Results.length)
                return null;
            return state.streamIndex + 1;
        },
        groupedTags(state) {
            let res = {
                tag: [],
                service: [],
                mark: [],
                generated: [],
            };
            if (state.tags != null) {
                for (const tag of state.tags) {
                    const type = tag.Name.split("/", 1)[0];
                    if (type in res)
                        res[type].push(tag);
                    else
                        console.log(`Tag ${tag.Name} has unsupported type`)
                }
            }
            return res;
        },
    },
    actions: {
        searchStreamsNew({ commit }, { query, page }) {
            if (!page) page = 0;
            commit('setStreams', { query, page, running: true, error: null, result: null });
            APIClient.searchStreams(query, page).then((data) => {
                if (!data.Error)
                    commit('setStreams', { query, page, running: false, error: null, result: data });
                else commit('setStreams', { query, page, running: false, error: data.Error, result: null });
            }).catch((err) => {
                commit('setStreams', { query, page, running: false, error: err.response.data, result: null });
            })
        },
        fetchStreamNew({ commit }, { id }) {
            commit('setStream', { id, running: true, error: null, stream: null });
            APIClient.getStream(id).then((data) => {
                commit('setStream', { id, running: false, error: null, stream: data });
            }).catch((err) => {
                commit('setStream', { id, running: false, error: err.response.data, stream: null });
            })
        },
        fetchGraphNew({ commit }, { delta, aspects, tags, query, type }) {
            commit('setGraph', { delta, aspects, tags, query, type, running: true, error: null, graph: null });
            APIClient.getGraph(delta, aspects, tags, query).then((data) => {
                commit('setGraph', { delta, aspects, tags, query, type, running: false, error: null, graph: data });
            }).catch((err) => {
                commit('setGraph', { delta, aspects, tags, query, type, running: false, error: err.response.data, graph: null });
            })
        },
        switchSearchPage({ dispatch, state }, page) {
            dispatch('searchStreamsObject', { query: state.searchQuery, page: page - 1 });
        },
        searchStreams({ dispatch }, query) {
            dispatch('searchStreamsObject', { query: query, page: 0 });
        },
        searchStreamsObject({ commit }, obj) {
            commit('searchStarted', obj);
            APIClient.searchStreams(obj.query, obj.page).then((data) => {
                commit('resetSearchResponse', data);
            }).catch((data) => {
                commit('resetSearchResponse', {
                    Error: data,
                });
            })
        },
        getStream({ commit, state }, streamIndex) {
            commit('resetStreamIndex', streamIndex);
            var streamId = state.searchResponse.Results[streamIndex].Stream.ID;
            APIClient.getStream(streamId).then((data) => {
                commit('resetStreamData', data);
            })
        },
        updateStatus({ commit }) {
            APIClient.getStatus().then((data) => {
                commit('resetStatus', data);
            })
        },
        updateTags({ commit }) {
            APIClient.getTags().then((data) => {
                commit('resetTags', data);
            })
        },
        updatePcaps({ commit }) {
            APIClient.getPcaps().then((data) => {
                commit('resetPcaps', data);
            })
        },
        async addTag({ commit, dispatch }, { name, query, color }) {
            commit('resetTagAddStatus', { inProgress: true })
            return APIClient.addTag(name, query, color).then(() => {
                commit('resetTagAddStatus', { inProgress: false })
                dispatch('updateTags');
            }).catch((err) => {
                commit('resetTagAddStatus', { error: err, inProgress: false })
                throw err.response.data;
            })
        },
        async delTag({ commit, dispatch }, name) {
            commit('resetTagDelStatus', { inProgress: true })
            return APIClient.delTag(name).then(() => {
                commit('resetTagDelStatus', { inProgress: false })
                commit('updateMark', { name, value: false })
                dispatch('updateTags');
            }).catch((err) => {
                commit('resetTagDelStatus', { error: err, inProgress: false })
                throw err.response.data;
            })
        },
        async changeTagColor({ dispatch }, { name, color }) {
            return APIClient.changeTagColor(name, color).catch((err) => {
                throw err.response.data;
            }).then(() => {
                dispatch('updateTags');
            });
        },
        updateGraph({ commit }, { delta, aspects, tags, query }) {
            APIClient.getGraph(delta, aspects, tags, query).then((data) => {
                commit('resetGraphData', data);
            })
        },
        async markTagNew({ dispatch, commit }, { name, streams, color }) {
            commit('resetMarkTagNewStatus', { inProgress: true, error: null });
            return APIClient.markTagNew(name, streams, color).catch((err) => {
                commit('resetMarkTagNewStatus', { inProgress: false, error: err.response.data });
                throw err.response.data;
            }).then(() => {
                commit('resetMarkTagNewStatus', { inProgress: false, error: null });
                commit('updateMark', { name, streams, value: true })
                dispatch('updateTags');
            });
        },
        async markTagAdd({ dispatch, commit }, { name, streams }) {
            commit('resetMarkTagUpdateStatus', { inProgress: true, error: null });
            return APIClient.markTagAdd(name, streams).catch((err) => {
                commit('resetMarkTagUpdateStatus', { inProgress: false, error: err.response.data });
                throw err.response.data;
            }).then(() => {
                commit('resetMarkTagUpdateStatus', { inProgress: false, error: null });
                commit('updateMark', { name, streams, value: true })
                dispatch('updateTags');
            });
        },
        async markTagDel({ dispatch, commit }, { name, streams }) {
            commit('resetMarkTagUpdateStatus', { inProgress: true, error: null });
            return APIClient.markTagDel(name, streams).catch((err) => {
                commit('resetMarkTagUpdateStatus', { inProgress: false, error: err.response.data });
                throw err.response.data;
            }).then(() => {
                commit('resetMarkTagUpdateStatus', { inProgress: false, error: null });
                commit('updateMark', { name, streams, value: false })
                dispatch('updateTags');
            });
        },
    }
});

export default store;
