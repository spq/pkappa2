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
    getTags() {
        return this.perform('get', `/tags`);
    },
    addTag(name, query) {
        console.log(name, query);
        return this.perform('put', `/tags`, query, { name });
    },
    delTag(name) {
        return this.perform('delete', `/tags`, null, { name });
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
