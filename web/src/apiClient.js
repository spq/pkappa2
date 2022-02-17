import axios from 'axios';

const client = axios.create({
    baseURL: '/api/',
    json: true
});

const APIClient = {
    searchStreams(query, page) {
        return this.perform('post', '/search.json', query, { page });
    },
    getStream(streamId) {
        return this.perform('get', `/stream/${streamId}.json`);
    },
    getStatus() {
        return this.perform('get', `/status.json`);
    },
    getPcaps() {
        return this.perform('get', `/pcaps.json`);
    },
    getTags() {
        return this.perform('get', `/tags`);
    },
    addTag(name, query, color) {
        return this.perform('put', `/tags`, query, { name, color });
    },
    delTag(name) {
        return this.perform('delete', `/tags`, null, { name });
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
        return this.perform('get', '/graph.json', null, params);
    },
    markTagNew(name, streams, color) {
        if (streams.length == 0) streams = [-1];
        return this.addTag(name, `id:${streams.join(',')}`, color)
    },
    markTagAdd(name, streams) {
        const params = new URLSearchParams();
        params.append("name", name)
        params.append("method", "mark_add")
        for (const s of streams) {
            params.append("stream", s)
        }
        return this.perform('patch', `/tags`, null, params);
    },
    markTagDel(name, streams) {
        const params = new URLSearchParams();
        params.append("name", name)
        params.append("method", "mark_del")
        for (const s of streams) {
            params.append("stream", s)
        }
        return this.perform('patch', `/tags`, null, params);
    },

    async perform(method, resource, data, params) {
        return client({
            method,
            url: resource,
            data,
            params,
        }).then(req => {
            return req.data;
        })
    }
}

export default APIClient;
