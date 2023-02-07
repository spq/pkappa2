import axios from "axios";

const client = axios.create({
  baseURL: "/api/",
  json: true,
});

const APIClient = {
  searchStreams(query, page) {
    return this.perform("post", "/search.json", query, { page });
  },
  getStream(streamId, converter) {
    return this.perform("get", `/stream/${streamId}.json`, null, { converter });
  },
  getStatus() {
    return this.perform("get", `/status.json`);
  },
  getPcaps() {
    return this.perform("get", `/pcaps.json`);
  },
  getConverters() {
    return this.perform("get", `/converters`);
  },
  getConverterStderr(converter) {
    return this.perform("get", `/converters/stderr/${converter}`);
  },
  resetConverter(converter) {
    return this.perform("delete", `/converters/${converter}`);
  },
  getTags() {
    return this.perform("get", `/tags`);
  },
  addTag(name, query, color) {
    return this.perform("put", `/tags`, query, { name, color });
  },
  delTag(name) {
    return this.perform("delete", `/tags`, null, { name });
  },
  changeTagColor(name, color) {
    const params = new URLSearchParams();
    params.append("name", name);
    params.append("method", "change_color");
    params.append("color", color);
    return this.perform("patch", `/tags`, null, params);
  },
  getGraph(delta, aspects, tags, query) {
    const params = new URLSearchParams();
    params.append("delta", delta);
    for (const a of aspects) {
      params.append("aspect", a);
    }
    for (const t of tags) {
      params.append("tag", t);
    }
    if (query) {
      params.append("query", query);
    }
    return this.perform("get", "/graph.json", null, params);
  },
  markTagNew(name, streams, color) {
    if (streams.length == 0) streams = [-1];
    return this.addTag(name, `id:${streams.join(",")}`, color);
  },
  converterTagSet(tagName, converters) {
    const params = new URLSearchParams();
    params.append("name", tagName);
    params.append("method", "converter_set");
    for (const c of converters) {
      params.append("converters", c);
    }
    return this.perform("patch", `/tags`, null, params);
  },
  markTagAdd(name, streams) {
    const params = new URLSearchParams();
    params.append("name", name);
    params.append("method", "mark_add");
    for (const s of streams) {
      params.append("stream", s);
    }
    return this.perform("patch", `/tags`, null, params);
  },
  markTagDel(name, streams) {
    const params = new URLSearchParams();
    params.append("name", name);
    params.append("method", "mark_del");
    for (const s of streams) {
      params.append("stream", s);
    }
    return this.perform("patch", `/tags`, null, params);
  },

  async perform(method, resource, data, params) {
    return client({
      method,
      url: resource,
      data,
      params,
    }).then((req) => {
      return req.data;
    });
  },
};

export default APIClient;
