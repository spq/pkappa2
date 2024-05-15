import Vue from "vue";
import Vuex from "vuex";

import axios from "axios";
import APIClient from "./apiClient";
import {
  ConverterStatistics,
  GraphResponse,
  PcapInfo,
  SearchResult,
  Statistics,
  StreamData,
  TagInfo,
} from "./apiClient";
import { MyStore } from "./vuex";

Vue.use(Vuex);

type StreamsState = {
  query: string | null;
  page: number | null;
  running: boolean;
  error: string | null;
  result: SearchResult | null;
};

type StreamState = {
  id: number | null;
  running: boolean;
  error: string | null;
  stream: StreamData | null;
};

export type GraphType =
  | "Active Connections"
  | "Started Connections"
  | "Finished Connections"
  | "Total Traffic"
  | "Average Traffic"
  | "Total Duration"
  | "Average Duration";

type GraphState = {
  type: GraphType | null;
  delta: number | null;
  aspects: string[] | null;
  tags: string[] | null;
  query: string | null;
  running: boolean;
  error: string | null;
  graph: GraphResponse | null;
};

export interface State {
  status: Statistics | null;
  pcaps: PcapInfo[] | null;
  tags: TagInfo[] | null;
  converters: ConverterStatistics[] | null;
  streams: StreamsState;
  stream: StreamState;
  graph: GraphState;
}

type UpdateMarkParam = {
  name: string;
  streams: number[];
  value: boolean;
};

type SearchStreamsParam = {
  query: string;
  page: number;
};

type FetchStreamParam = {
  id: number;
  converter: string;
};

type FetchGraphParam = {
  delta: string;
  aspects: string[];
  tags: string[];
  query: string;
  type: string;
};

type AddTagParam = {
  name: string;
  query: string;
  color: string;
};

type ChangeTagColorParam = {
  name: string;
  color: string;
};

type SetTagConvertersParam = {
  name: string;
  converters: string[];
};

type AddMarkTagParam = {
  name: string;
  streams: number[];
  color: string;
};

type ChangeMarkTagParam = {
  name: string;
  streams: number[];
};

type Event = {
  Type: string;
  Tag: TagInfo | null;
};

export interface Getters {
  groupedTags: { [key: string]: TagInfo[] };
}

type GettersDefinition = {
  [P in keyof Getters]: (state: State, getters: Getters) => Getters[P];
};

export function handleAxiosDefaultError(err: unknown) {
  if (axios.isAxiosError<string, unknown>(err) && err.response !== undefined)
    throw err.response.data;
  else throw err;
}

export const getters: GettersDefinition = {
  groupedTags(state: State) {
    const res: { [key: string]: TagInfo[] } = {
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
};

const ws = (store: MyStore) => {
  let reconnectTimeout = 125;
  const connect = () => {
    const l = window.location;
    const url = `ws${l.protocol.slice(4)}//${l.host}/ws`;
    const ws = new WebSocket(url);
    ws.onopen = () => {
      reconnectTimeout = 125;
    };
    ws.onerror = (err) => {
      console.error("WebSocket error:", err);
      ws.close();
    };
    ws.onclose = () => {
      console.log(
        `WebSocket closed, reconnecting in ${
          reconnectTimeout < 1000
            ? `1/${1000 / reconnectTimeout}`
            : reconnectTimeout / 1000.0
        }s`
      );
      setTimeout(() => {
        if (reconnectTimeout < 10000) reconnectTimeout *= 2;
        connect();
      }, reconnectTimeout);
    };
    ws.onmessage = (event) => {
      const e: Event = JSON.parse(event.data as string) as Event;
      switch (e.Type) {
        case "tagAdded":
          store.commit("addTag", e.Tag);
          break;
        case "tagDeleted":
          store.commit("delTag", e.Tag?.Name);
          break;
        case "tagUpdated":
        case "tagEvaluated":
          store.commit("updateTag", e.Tag);
          break;
        default:
          console.log(`Unhandled event type: ${e.Type}`);
          break;
      }
    };
  };
  connect();
};

const store = new Vuex.Store({
  state: {
    status: null,
    pcaps: null,

    tags: null,
    converters: null,

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
    setStreams(state: State, obj: StreamsState) {
      state.streams = obj;
    },
    setStream(state: State, obj: StreamState) {
      state.stream = obj;
    },
    setGraph(state: State, obj: GraphState) {
      state.graph = obj;
    },
    updateMark(state: State, { name, streams, value }: UpdateMarkParam) {
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
    setStatus(state: State, status: Statistics) {
      state.status = status;
    },
    setTags(state: State, tags: TagInfo[]) {
      state.tags = tags;
    },
    addTag(state: State, tag: TagInfo) {
      if (state.tags != null) state.tags.push(tag);
    },
    delTag(state: State, tagName: string) {
      if (state.tags != null)
        state.tags = state.tags.filter((tag) => tag.Name != tagName);
    },
    updateTag(state: State, tag: TagInfo) {
      if (state.tags != null)
        state.tags = state.tags.map((t) => (t.Name == tag.Name ? tag : t));
    },
    setConverters(state: State, converters: ConverterStatistics[]) {
      state.converters = converters;
    },
    setPcaps(state: State, pcaps: PcapInfo[]) {
      state.pcaps = pcaps;
    },
  },
  getters: getters,
  actions: {
    searchStreams({ commit }, { query, page }: SearchStreamsParam) {
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
          if ("Error" in data)
            commit("setStreams", {
              query,
              page,
              running: false,
              error: data.Error,
              result: null,
            });
          else
            commit("setStreams", {
              query,
              page,
              running: false,
              error: null,
              result: data,
            });
        })
        .catch((err) => {
          if (axios.isCancel(err)) return;
          if (
            axios.isAxiosError<string, unknown>(err) &&
            err.response !== undefined
          )
            commit("setStreams", {
              query,
              page,
              running: false,
              error: err.response.data,
              result: null,
            });
          else throw err;
        });
    },
    fetchStream({ commit }, { id, converter }: FetchStreamParam) {
      commit("setStream", { id, running: true, error: null, stream: null });
      APIClient.getStream(id, converter)
        .then((data) => {
          commit("setStream", {
            id,
            running: false,
            error: null,
            stream: data,
          });
        })
        .catch((err) => {
          if (
            axios.isAxiosError<string, unknown>(err) &&
            err.response !== undefined
          )
            commit("setStream", {
              id,
              running: false,
              error: err.response.data,
              stream: null,
            });
          else throw err;
        });
    },
    fetchGraph(
      { commit },
      { delta, aspects, tags, query, type }: FetchGraphParam
    ) {
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
          if (axios.isCancel(err)) return;
          if (
            axios.isAxiosError<string, unknown>(err) &&
            err.response !== undefined
          )
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
          else throw err;
        });
    },
    updateStatus({ commit }) {
      APIClient.getStatus()
        .then((data) => {
          commit("setStatus", data);
        })
        .catch(handleAxiosDefaultError);
    },
    updateTags({ commit }) {
      APIClient.getTags()
        .then((data) => {
          commit("setTags", data);
        })
        .catch(handleAxiosDefaultError);
    },
    updateConverters({ commit }) {
      APIClient.getConverters()
        .then((data) => {
          commit("setConverters", data);
        })
        .catch(handleAxiosDefaultError);
    },
    updatePcaps({ commit }) {
      APIClient.getPcaps()
        .then((data) => {
          commit("setPcaps", data);
        })
        .catch(handleAxiosDefaultError);
    },
    async addTag({ dispatch }, { name, query, color }: AddTagParam) {
      return APIClient.addTag(name, query, color)
        .then(() => {
          dispatch("updateTags").catch((err) => {
            throw err;
          });
        })
        .catch(handleAxiosDefaultError);
    },
    async delTag({ commit, dispatch }, { name }: { name: string }) {
      return APIClient.delTag(name)
        .then(() => {
          commit("updateMark", { name, value: false });
          dispatch("updateTags").catch((err) => {
            throw err;
          });
        })
        .catch(handleAxiosDefaultError);
    },
    async changeTagColor({ dispatch }, { name, color }: ChangeTagColorParam) {
      return APIClient.changeTagColor(name, color)
        .then(() => {
          dispatch("updateTags").catch((err) => {
            throw err;
          });
        })
        .catch(handleAxiosDefaultError);
    },
    async setTagConverters(
      { dispatch },
      { name, converters }: SetTagConvertersParam
    ) {
      return APIClient.converterTagSet(name, converters)
        .then(() => {
          dispatch("updateTags").catch((err) => {
            throw err;
          });
        })
        .catch(handleAxiosDefaultError);
    },
    async resetConverter({ dispatch }, { name }: { name: string }) {
      return APIClient.resetConverter(name)
        .then(() => {
          dispatch("updateConverters").catch((err) => {
            throw err;
          });
        })
        .catch(handleAxiosDefaultError);
    },
    async markTagNew(
      { dispatch, commit },
      { name, streams, color }: AddMarkTagParam
    ) {
      return APIClient.markTagNew(name, streams, color)
        .then(() => {
          commit("updateMark", { name, streams, value: true });
          dispatch("updateTags").catch((err) => {
            throw err;
          });
        })
        .catch(handleAxiosDefaultError);
    },
    async markTagAdd(
      { dispatch, commit },
      { name, streams }: ChangeMarkTagParam
    ) {
      return APIClient.markTagAdd(name, streams)
        .then(() => {
          commit("updateMark", { name, streams, value: true });
          dispatch("updateTags").catch((err) => {
            throw err;
          });
        })
        .catch(handleAxiosDefaultError);
    },
    async markTagDel(
      { dispatch, commit },
      { name, streams }: ChangeMarkTagParam
    ) {
      return APIClient.markTagDel(name, streams)
        .then(() => {
          commit("updateMark", { name, streams, value: false });
          dispatch("updateTags").catch((err) => {
            throw err;
          });
        })
        .catch(handleAxiosDefaultError);
    },
  },
  plugins: [ws],
});

export default store;
export const useStore = () => store as MyStore;
