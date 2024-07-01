// Generated automatically by nearley, version 2.20.1
// http://github.com/Hardmath123/nearley
// Bypasses TS6133. Allow declared but unused functions.
// @ts-ignore
function id(d: any[]): any { return d[0]; }
declare var ws: any;
declare var kw_or: any;
declare var kw_and: any;
declare var kw_then: any;
declare var negation: any;
declare var lparen: any;
declare var rparen: any;
declare var subquery: any;
declare var kw: any;
declare var converter: any;
declare var value: any;
declare var keyword_or_error: any;

/* eslint-disable */
/* prettier-ignore */
import moo from "moo"

const lexer = moo.compile({
    ws: {match: /[ \t\n\r]+/, lineBreaks: true},
    value: [
        {match: /[:=]"(?:[^"]*|"")*"/, value: x => x.slice(2, -1)},
        {match: /[:=]"(?:[^"]*|"")*/, value: x => x.slice(2)},
        {match: /[:=](?:(?:[^"\\ \t\n\r]|\\.)(?:[^\\ \t\n\r]|\\.)*)?(?:[^)\\ \t\n\r]|\\.)/, value: x => x.slice(1)},
        {match: /[:=]/, value: _ => ""},
    ],
    lparen: '(',
    rparen: ')',
    subquery: {match: /@[a-z0-9]+:/, value: x => x.slice(1, -1)},
    converter: {match: /\.[a-z0-9]*/, value: x => x.slice(1)},
    negation: /[!-]/,
    keyword_or_error: {match: /[a-zA-Z]+/, error: true, type: moo.keywords({
        kw: ['id', 'tag', 'service', 'mark', 'generated', 'protocol', 'ftime', 'ltime', 'time', 'cdata', 'sdata', 'data', 'cport', 'sport', 'port', 'chost', 'shost', 'host', 'cbytes', 'sbytes', 'bytes', 'sort', 'limit', 'group'],
        'kw_or': 'or',
        'kw_and': 'and',
        'kw_then': 'then',
    })},
});

export interface QueryElement {
  type: string;
};

export interface LogicQueryElement extends QueryElement {
  type: "logic";
  op: "or" | "and" | "sequence";
  expressions: QueryElement[];
}

export interface SubexpressionQueryElement extends QueryElement {
  type: "not" | "subquery" | "error";
  expression: QueryElement;
}

export interface ExpressionQueryElement extends QueryElement {
  type: "expression";
  subquery_var?: moo.Token;
  keyword: moo.Token;
  converter?: moo.Token;
  value?: moo.Token;
}

export function isLogicExpression(obj: QueryElement): obj is LogicQueryElement {
  return obj.type === "logic";
}

export function isSubExpression(obj: QueryElement): obj is SubexpressionQueryElement {
  return ["not", "subquery", "error"].includes(obj.type);
}

export function isExpression(obj: QueryElement): obj is ExpressionQueryElement {
  return obj.type === "expression";
}

interface NearleyToken {
  value: any;
  [key: string]: any;
};

interface NearleyLexer {
  reset: (chunk: string, info: any) => void;
  next: () => NearleyToken | undefined;
  save: () => any;
  formatError: (token: never) => string;
  has: (tokenType: string) => boolean;
};

interface NearleyRule {
  name: string;
  symbols: NearleySymbol[];
  postprocess?: (d: any[], loc?: number, reject?: {}) => any;
};

type NearleySymbol = string | { literal: any } | { test: (token: any) => boolean };

interface Grammar {
  Lexer: NearleyLexer | undefined;
  ParserRules: NearleyRule[];
  ParserStart: string;
};

const grammar: Grammar = {
  Lexer: lexer,
  ParserRules: [
    {"name": "queryRoot$ebnf$1", "symbols": [(lexer.has("ws") ? {type: "ws"} : ws)], "postprocess": id},
    {"name": "queryRoot$ebnf$1", "symbols": [], "postprocess": () => null},
    {"name": "queryRoot", "symbols": ["queryOrCondition", "queryRoot$ebnf$1"], "postprocess": id},
    {"name": "queryOrCondition", "symbols": ["queryOrCondition", (lexer.has("ws") ? {type: "ws"} : ws), (lexer.has("kw_or") ? {type: "kw_or"} : kw_or), (lexer.has("ws") ? {type: "ws"} : ws), "queryAndCondition"], "postprocess": (d) => d.length > 1 ? {'type': 'logic', 'op': 'or', 'expressions': [d[0], d[4]]} : d[0]},
    {"name": "queryOrCondition", "symbols": ["queryAndCondition"], "postprocess": id},
    {"name": "queryAndCondition$ebnf$1$subexpression$1", "symbols": [(lexer.has("kw_and") ? {type: "kw_and"} : kw_and), (lexer.has("ws") ? {type: "ws"} : ws)]},
    {"name": "queryAndCondition$ebnf$1", "symbols": ["queryAndCondition$ebnf$1$subexpression$1"], "postprocess": id},
    {"name": "queryAndCondition$ebnf$1", "symbols": [], "postprocess": () => null},
    {"name": "queryAndCondition", "symbols": ["queryAndCondition", (lexer.has("ws") ? {type: "ws"} : ws), "queryAndCondition$ebnf$1", "queryThenCondition"], "postprocess": (d) => d.length > 1 ? {'type': 'logic', 'op': 'and', 'expressions': [d[0], d[3]]} : d[0]},
    {"name": "queryAndCondition", "symbols": ["queryThenCondition"], "postprocess": id},
    {"name": "queryThenCondition", "symbols": ["queryThenCondition", (lexer.has("ws") ? {type: "ws"} : ws), (lexer.has("kw_then") ? {type: "kw_then"} : kw_then), (lexer.has("ws") ? {type: "ws"} : ws), "queryCondition"], "postprocess": (d) => d.length > 1 ? {'type': 'logic', 'op': 'sequence', 'expressions': [d[0], d[4]]} : d[0]},
    {"name": "queryThenCondition", "symbols": ["queryCondition"], "postprocess": id},
    {"name": "queryCondition", "symbols": [(lexer.has("negation") ? {type: "negation"} : negation), "queryCondition"], "postprocess": function(d) {return {'type': 'not', 'expression': d[1]};}},
    {"name": "queryCondition$ebnf$1", "symbols": [(lexer.has("ws") ? {type: "ws"} : ws)], "postprocess": id},
    {"name": "queryCondition$ebnf$1", "symbols": [], "postprocess": () => null},
    {"name": "queryCondition$ebnf$2", "symbols": [(lexer.has("ws") ? {type: "ws"} : ws)], "postprocess": id},
    {"name": "queryCondition$ebnf$2", "symbols": [], "postprocess": () => null},
    {"name": "queryCondition", "symbols": [(lexer.has("lparen") ? {type: "lparen"} : lparen), "queryCondition$ebnf$1", "queryOrCondition", "queryCondition$ebnf$2", (lexer.has("rparen") ? {type: "rparen"} : rparen)], "postprocess": function(d) {return {'type': 'subquery', 'expression': d[2]};}},
    {"name": "queryCondition$ebnf$3", "symbols": [(lexer.has("subquery") ? {type: "subquery"} : subquery)], "postprocess": id},
    {"name": "queryCondition$ebnf$3", "symbols": [], "postprocess": () => null},
    {"name": "queryCondition$ebnf$4", "symbols": [(lexer.has("converter") ? {type: "converter"} : converter)], "postprocess": id},
    {"name": "queryCondition$ebnf$4", "symbols": [], "postprocess": () => null},
    {"name": "queryCondition$ebnf$5", "symbols": [(lexer.has("value") ? {type: "value"} : value)], "postprocess": id},
    {"name": "queryCondition$ebnf$5", "symbols": [], "postprocess": () => null},
    {"name": "queryCondition", "symbols": ["queryCondition$ebnf$3", (lexer.has("kw") ? {type: "kw"} : kw), "queryCondition$ebnf$4", "queryCondition$ebnf$5"], "postprocess": function(d) {return {'type': 'expression', 'subquery_var':d[0], 'keyword':d[1], 'converter': d[2], 'value': d[3]};}},
    {"name": "queryCondition$ebnf$6", "symbols": [(lexer.has("converter") ? {type: "converter"} : converter)], "postprocess": id},
    {"name": "queryCondition$ebnf$6", "symbols": [], "postprocess": () => null},
    {"name": "queryCondition$ebnf$7", "symbols": [(lexer.has("value") ? {type: "value"} : value)], "postprocess": id},
    {"name": "queryCondition$ebnf$7", "symbols": [], "postprocess": () => null},
    {"name": "queryCondition$ebnf$8", "symbols": [(lexer.has("ws") ? {type: "ws"} : ws)], "postprocess": id},
    {"name": "queryCondition$ebnf$8", "symbols": [], "postprocess": () => null},
    {"name": "queryCondition", "symbols": [(lexer.has("keyword_or_error") ? {type: "keyword_or_error"} : keyword_or_error), "queryCondition$ebnf$6", "queryCondition$ebnf$7", "queryCondition$ebnf$8"], "postprocess": function(d) {return {'type': 'error', 'expression': d[0]};}}
  ],
  ParserStart: "queryRoot",
};

export default grammar;
