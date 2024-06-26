import nearley, { Parser } from "nearley";
import grammar from "./query";

interface QueryElement {
  type: string;
  op?: string;
  expressions?: QueryElement[];
  keyword?: {
    value: string;
  };
  [key: string]: any;
}

interface QueryElementValue {
  pieces: { [key: string]: string };
  start: number;
  len: number;
};

const queryGrammar: nearley.Grammar = nearley.Grammar.fromCompiled(grammar);

export default function analyze(query: string): { [key: string]: QueryElementValue } {
  const parser: Parser = new nearley.Parser(queryGrammar);
  try {
    parser.feed(query);
  } catch (parseError: any) {
    console.log("Error at character " + parseError.offset);
    return {};
  }

  const result: { [key: string]: QueryElementValue } = {};
  const elements: QueryElement[] = [...parser.results];
  while (true) {
    const elem = elements.pop();
    if (elem === undefined) break;
    if (
      elem.type == "logic" &&
      elem.op == "and" &&
      elem.expressions !== undefined
    ) {
      elements.push(...elem.expressions);
      continue;
    }
    if (elem.type != "expression" || elem.keyword === undefined) continue;
    if (!["sort", "limit", "ltime"].includes(elem.keyword.value)) continue;
    let pieces: { [key: string]: string } = {};
    var start: number | null = null;
    var end: number | null = null;
    for (const [k, v] of Object.entries(elem)) {
      if (k == "type") continue;
      if (v == null) continue;
      pieces[k] = v.value;
      if (start == null || start > v.col) {
        start = v.col;
      }
      if (end == null || end < v.col + v.text.length) {
        end = v.col + v.text.length;
      }
    }
    if (start == null || end == null) continue;
    result[elem.keyword.value] = {pieces, start, len: end - start};
  }
  return result;
}
