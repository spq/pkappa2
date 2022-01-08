import Vue from 'vue';
import VueRouter from 'vue-router';

import HomeOld from './components/Home';
import Base from './components/new/Base';
import Home from './components/new/Home';
import Status from './components/new/Status';
import Pcaps from './components/new/Pcaps';
import Tags from './components/new/Tags';
import Graph from './components/new/Graph';
import Results from './components/new/Results';
import Stream from './components/new/Stream';

Vue.use(VueRouter);

export default new VueRouter({
    mode: 'hash',
    routes: [
        { path: '/old', component: HomeOld },
        {
            path: '/',
            component: Base,
            children: [
                {
                    path: '',
                    name: 'home',
                    component: Home,
                },
                {
                    path: 'status',
                    name: 'status',
                    component: Status,
                },
                {
                    path: 'pcaps',
                    name: 'pcaps',
                    component: Pcaps,
                },
                {
                    path: 'tags',
                    name: 'tags',
                    component: Tags,
                },
                {
                    path: 'graph',
                    name: 'graph',
                    component: Graph,
                    props: route => ({ searchTerm: route.query.q })
                },
                {
                    path: 'search',
                    name: 'search',
                    component: Results,
                    props: route => ({ searchTerm: route.query.q, searchPage: route.query.p })
                },
                {
                    path: 'stream/:streamId(\\d+)',
                    name: 'stream',
                    component: Stream,
                    props: route => ({ searchTerm: route.query.q, searchPage: route.query.p })
                },
            ],
        },
    ]
});
