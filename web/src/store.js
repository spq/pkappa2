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

        tags: null,
        tagAddStatus: null,
        tagDelStatus: null,

        graphData: null,

        markTagNewStatus: null,
    },
    mutations: {
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
        }
    },
    getters: {
        searchResponse(state) {
            return state.searchResponse;
        },
        searchRunning(state) {
            return state.searchRunning;
        },
        searchPage(state) {
            return state.searchPage;
        },
        streamData(state) {
            return state.streamData;
        },
        streamLoading(state) {
            return state.streamLoading;
        },
        status(state) {
            return state.status;
        },
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
    },
    actions: {
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
            var streamId = state.searchResponse.Results[streamIndex].ID;
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
        addTag({ commit, dispatch }, { name, query }) {
            commit('resetTagAddStatus', { inProgress: true })
            APIClient.addTag(name, query).then(() => {
                commit('resetTagAddStatus', { inProgress: false })
                dispatch('updateTags');
            }).catch((data) => {
                commit('resetTagAddStatus', { error: data, inProgress: false })
            })
        },
        delTag({ commit, dispatch }, name) {
            commit('resetTagDelStatus', { inProgress: true })
            APIClient.delTag(name).then(() => {
                commit('resetTagDelStatus', { inProgress: false })
                dispatch('updateTags');
            }).catch((data) => {
                commit('resetTagDelStatus', { error: data, inProgress: false })
            })
        },
        updateGraph({ commit }, { delta, aspects, tags }) {
            APIClient.getGraph(delta, aspects, tags).then((data) => {
                commit('resetGraphData', data);
            })
        },
        markTagNew({ dispatch, commit }, { name, streams }) {
            APIClient.markTagNew(name, streams).then(() => {
                commit('resetMarkTagNewStatus', { inProgress: false });
                dispatch('updateTags');
            }).catch((data) => {
                commit('resetMarkTagNewStatus', { error: data, inProgress: false });
            });
        },
        markTagAdd({ dispatch, commit }, { name, streams }) {
            APIClient.markTagAdd(name, streams).then(() => {
                commit('resetMarkTagUpdateStatus', { inProgress: false });
                dispatch('updateTags');
            }).catch((data) => {
                commit('resetMarkTagUpdateStatus', { error: data, inProgress: false });
            });
        },
        markTagDel({ dispatch, commit }, { name, streams }) {
            APIClient.markTagDel(name, streams).then(() => {
                commit('resetMarkTagUpdateStatus', { inProgress: false });
                dispatch('updateTags');
            }).catch((data) => {
                commit('resetMarkTagUpdateStatus', { error: data, inProgress: false });
            });
        },
    }
});

export default store;
