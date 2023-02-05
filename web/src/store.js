import Vue from "vue";
import Vuex from "vuex";

import APIClient from "./apiClient";

Vue.use(Vuex);

const store = new Vuex.Store({
  state: {
    status: null,
    pcaps: null,

    tags: null,

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
      if (
        state.stream.stream != null &&
        (streams == undefined ||
          streams.includes(state.stream.stream.Stream.ID))
      ) {
        const s = state.stream.stream;
        const current = s.Tags.includes(name);
        if (value && !current) {
          s.Tags.push(name);
        } else if (current && !value) {
          s.Tags = s.Tags.filter((t) => t != name);
        }
      }
      if (state.streams.result != null) {
        for (const s of state.streams.result.Results) {
          if (streams != undefined && !streams.includes(s.Stream.ID)) continue;
          const current = s.Tags.includes(name);
          if (value && !current) {
            s.Tags.push(name);
          } else if (current && !value) {
            s.Tags = s.Tags.filter((t) => t != name);
          }
        }
      }
    },
    setStatus(state, status) {
      state.status = status;
    },
    setTags(state, tags) {
      state.tags = tags;
    },
    setPcaps(state, pcaps) {
      state.pcaps = pcaps;
    },
  },
  getters: {
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
          if (type in res) res[type].push(tag);
          else console.log(`Tag ${tag.Name} has unsupported type`);
        }
      }
      return res;
    },
  },
  actions: {
    searchStreams({ commit }, { query, page }) {
      if (!page) page = 0;
      commit("setStreams", {
        query,
        page,
        running: true,
        error: null,
        result: null,
      });
      APIClient.searchStreams(query, page)
        .then((data) => {
          if (!data.Error)
            commit("setStreams", {
              query,
              page,
              running: false,
              error: null,
              result: data,
            });
          else
            commit("setStreams", {
              query,
              page,
              running: false,
              error: data.Error,
              result: null,
            });
        })
        .catch((err) => {
          commit("setStreams", {
            query,
            page,
            running: false,
            error: err.response.data,
            result: null,
          });
        });
    },
    fetchStream({ commit }, { id }) {
      commit("setStream", { id, running: true, error: null, stream: null });
      APIClient.getStream(id)
        .then((data) => {
          commit("setStream", {
            id,
            running: false,
            error: null,
            stream: data,
          });
        })
        .catch((err) => {
          commit("setStream", {
            id,
            running: false,
            error: err.response.data,
            stream: null,
          });
        });
    },
    fetchGraph({ commit }, { delta, aspects, tags, query, type }) {
      commit("setGraph", {
        delta,
        aspects,
        tags,
        query,
        type,
        running: true,
        error: null,
        graph: null,
      });
      APIClient.getGraph(delta, aspects, tags, query)
        .then((data) => {
          commit("setGraph", {
            delta,
            aspects,
            tags,
            query,
            type,
            running: false,
            error: null,
            graph: data,
          });
        })
        .catch((err) => {
          commit("setGraph", {
            delta,
            aspects,
            tags,
            query,
            type,
            running: false,
            error: err.response.data,
            graph: null,
          });
        });
    },
    updateStatus({ commit }) {
      APIClient.getStatus().then((data) => {
        commit("setStatus", data);
      });
    },
    updateTags({ commit }) {
      APIClient.getTags().then((data) => {
        commit("setTags", data);
      });
    },
    updatePcaps({ commit }) {
      APIClient.getPcaps().then((data) => {
        commit("setPcaps", data);
      });
    },
    async addTag({ dispatch }, { name, query, color }) {
      return APIClient.addTag(name, query, color)
        .then(() => {
          dispatch("updateTags");
        })
        .catch((err) => {
          throw err.response.data;
        });
    },
    async delTag({ commit, dispatch }, name) {
      return APIClient.delTag(name)
        .then(() => {
          commit("updateMark", { name, value: false });
          dispatch("updateTags");
        })
        .catch((err) => {
          throw err.response.data;
        });
    },
    async changeTagColor({ dispatch }, { name, color }) {
      return APIClient.changeTagColor(name, color)
        .catch((err) => {
          throw err.response.data;
        })
        .then(() => {
          dispatch("updateTags");
        });
    },
    async markTagNew({ dispatch, commit }, { name, streams, color }) {
      return APIClient.markTagNew(name, streams, color)
        .catch((err) => {
          throw err.response.data;
        })
        .then(() => {
          commit("updateMark", { name, streams, value: true });
          dispatch("updateTags");
        });
    },
    async markTagAdd({ dispatch, commit }, { name, streams }) {
      return APIClient.markTagAdd(name, streams)
        .catch((err) => {
          throw err.response.data;
        })
        .then(() => {
          commit("updateMark", { name, streams, value: true });
          dispatch("updateTags");
        });
    },
    async markTagDel({ dispatch, commit }, { name, streams }) {
      return APIClient.markTagDel(name, streams)
        .catch((err) => {
          throw err.response.data;
        })
        .then(() => {
          commit("updateMark", { name, streams, value: false });
          dispatch("updateTags");
        });
    },
  },
});

export default store;
