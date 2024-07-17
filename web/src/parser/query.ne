# nearleyc query.ne -o query.ts
@preprocessor esmodule
@preprocessor typescript
@{%
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
%}

@lexer lexer

# Rules
queryRoot -> queryOrCondition %ws:? {% id %}
queryOrCondition ->
    queryOrCondition %ws %kw_or %ws queryAndCondition {% (d) => d.length > 1 ? {'type': 'logic', 'op': 'or', 'expressions': [d[0], d[4]]} : d[0] %}
    | queryAndCondition {% id %}
queryAndCondition ->
    queryAndCondition %ws (%kw_and %ws):? queryThenCondition {% (d) => d.length > 1 ? {'type': 'logic', 'op': 'and', 'expressions': [d[0], d[3]]} : d[0] %}
    | queryThenCondition {% id %}
queryThenCondition ->
    queryThenCondition %ws %kw_then %ws queryCondition {% (d) => d.length > 1 ? {'type': 'logic', 'op': 'sequence', 'expressions': [d[0], d[4]]} : d[0] %}
    | queryCondition {% id %}
queryCondition ->
    %negation queryCondition  {% function(d) {return {'type': 'not', 'expression': d[1]};} %}
    | %lparen %ws:? queryOrCondition %ws:? %rparen {% function(d) {return {'type': 'subquery', 'expression': d[2]};} %}
    | %subquery:? %kw %converter:? %value:? {% function(d) {return {'type': 'expression', 'subquery_var':d[0], 'keyword':d[1], 'converter': d[2], 'value': d[3]};} %}
    | %keyword_or_error %converter:? %value:? %ws:? {% function(d) {return {'type': 'error', 'expression': d[0]};} %}
