// Generated automatically by nearley, version 2.20.1
// http://github.com/Hardmath123/nearley
(function () {
    function id(x) { return x[0]; }

    /* eslint-disable */
    const moo = require("moo");

    const lexer = moo.compile({
        ws: { match: /[ \t\n\r]+/, lineBreaks: true },
        value: [
            { match: /[:=]"(?:[^"]*|"")*"/, value: x => x.slice(2, -1) },
            { match: /[:=]"(?:[^"]*|"")*/, value: x => x.slice(2) },
            { match: /[:=](?:(?:[^"\\ \t\n\r]|\\.)(?:[^\\ \t\n\r]|\\.)*)?(?:[^)\\ \t\n\r]|\\.)/, value: x => x.slice(1) },
            { match: /[:=]/, value: () => null },
        ],
        lparen: '(',
        rparen: ')',
        subquery: { match: /@[a-z0-9]+:/, value: x => x.slice(1, -1) },
        negation: /[!-]/,
        keyword_or_error: {
            match: /[a-zA-Z]+/, error: true, type: moo.keywords({
                kw: ['id', 'tag', 'service', 'mark', 'protocol', 'ftime', 'ltime', 'time', 'cdata', 'sdata', 'data', 'cport', 'sport', 'port', 'chost', 'shost', 'host', 'cbytes', 'sbytes', 'bytes', 'sort', 'limit', 'group'],
                'kw_or': 'or',
                'kw_and': 'and',
                'kw_then': 'then',
            })
        },
    });
    var grammar = {
        Lexer: lexer,
        ParserRules: [
            { "name": "queryRoot$ebnf$1", "symbols": [(lexer.has("ws") ? { type: "ws" } : ws)], "postprocess": id },
            { "name": "queryRoot$ebnf$1", "symbols": [], "postprocess": function (d) { return null; } },
            { "name": "queryRoot", "symbols": ["queryOrCondition", "queryRoot$ebnf$1"], "postprocess": id },
            { "name": "queryOrCondition", "symbols": ["queryOrCondition", (lexer.has("ws") ? { type: "ws" } : ws), (lexer.has("kw_or") ? { type: "kw_or" } : kw_or), (lexer.has("ws") ? { type: "ws" } : ws), "queryAndCondition"], "postprocess": (d) => d.length > 1 ? { 'type': 'logic', 'op': 'or', 'expressions': [d[0], d[4]] } : d[0] },
            { "name": "queryOrCondition", "symbols": ["queryAndCondition"], "postprocess": id },
            { "name": "queryAndCondition$ebnf$1$subexpression$1", "symbols": [(lexer.has("kw_and") ? { type: "kw_and" } : kw_and), (lexer.has("ws") ? { type: "ws" } : ws)] },
            { "name": "queryAndCondition$ebnf$1", "symbols": ["queryAndCondition$ebnf$1$subexpression$1"], "postprocess": id },
            { "name": "queryAndCondition$ebnf$1", "symbols": [], "postprocess": function (d) { return null; } },
            { "name": "queryAndCondition", "symbols": ["queryAndCondition", (lexer.has("ws") ? { type: "ws" } : ws), "queryAndCondition$ebnf$1", "queryThenCondition"], "postprocess": (d) => d.length > 1 ? { 'type': 'logic', 'op': 'and', 'expressions': [d[0], d[3]] } : d[0] },
            { "name": "queryAndCondition", "symbols": ["queryThenCondition"], "postprocess": id },
            { "name": "queryThenCondition", "symbols": ["queryThenCondition", (lexer.has("ws") ? { type: "ws" } : ws), (lexer.has("kw_then") ? { type: "kw_then" } : kw_then), (lexer.has("ws") ? { type: "ws" } : ws), "queryCondition"], "postprocess": (d) => d.length > 1 ? { 'type': 'logic', 'op': 'sequence', 'expressions': [d[0], d[4]] } : d[0] },
            { "name": "queryThenCondition", "symbols": ["queryCondition"], "postprocess": id },
            { "name": "queryCondition", "symbols": [(lexer.has("negation") ? { type: "negation" } : negation), "queryCondition"], "postprocess": function (d) { return { 'type': 'not', 'expression': d[1] }; } },
            { "name": "queryCondition$ebnf$1", "symbols": [(lexer.has("ws") ? { type: "ws" } : ws)], "postprocess": id },
            { "name": "queryCondition$ebnf$1", "symbols": [], "postprocess": function (d) { return null; } },
            { "name": "queryCondition$ebnf$2", "symbols": [(lexer.has("ws") ? { type: "ws" } : ws)], "postprocess": id },
            { "name": "queryCondition$ebnf$2", "symbols": [], "postprocess": function (d) { return null; } },
            { "name": "queryCondition", "symbols": [(lexer.has("lparen") ? { type: "lparen" } : lparen), "queryCondition$ebnf$1", "queryOrCondition", "queryCondition$ebnf$2", (lexer.has("rparen") ? { type: "rparen" } : rparen)], "postprocess": function (d) { return { 'type': 'subquery', 'expression': d[2] }; } },
            { "name": "queryCondition$ebnf$3", "symbols": [(lexer.has("subquery") ? { type: "subquery" } : subquery)], "postprocess": id },
            { "name": "queryCondition$ebnf$3", "symbols": [], "postprocess": function (d) { return null; } },
            { "name": "queryCondition$ebnf$4", "symbols": [(lexer.has("value") ? { type: "value" } : value)], "postprocess": id },
            { "name": "queryCondition$ebnf$4", "symbols": [], "postprocess": function (d) { return null; } },
            { "name": "queryCondition", "symbols": ["queryCondition$ebnf$3", (lexer.has("kw") ? { type: "kw" } : kw), "queryCondition$ebnf$4"], "postprocess": function (d) { return { 'type': 'expression', 'subquery_var': d[0], 'keyword': d[1], 'value': d[2] }; } },
            { "name": "queryCondition$ebnf$5", "symbols": [(lexer.has("value") ? { type: "value" } : value)], "postprocess": id },
            { "name": "queryCondition$ebnf$5", "symbols": [], "postprocess": function (d) { return null; } },
            { "name": "queryCondition$ebnf$6", "symbols": [(lexer.has("ws") ? { type: "ws" } : ws)], "postprocess": id },
            { "name": "queryCondition$ebnf$6", "symbols": [], "postprocess": function (d) { return null; } },
            { "name": "queryCondition", "symbols": [(lexer.has("keyword_or_error") ? { type: "keyword_or_error" } : keyword_or_error), "queryCondition$ebnf$5", "queryCondition$ebnf$6"], "postprocess": function (d) { return { 'type': 'error', 'expression': d[0] }; } }
        ]
        , ParserStart: "queryRoot"
    }
    if (typeof module !== 'undefined' && typeof module.exports !== 'undefined') {
        module.exports = grammar;
    } else {
        window.grammar = grammar;
    }
})();
