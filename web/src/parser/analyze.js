import nearley from "nearley";
import grammar from "./query";

const queryGrammar = nearley.Grammar.fromCompiled(grammar);

export default function analyze(query) {
  const parser = new nearley.Parser(queryGrammar);
  try {
    parser.feed(query);
  } catch (parseError) {
    console.log("Error at character " + parseError.offset);
    return {};
  }

  const result = {};
  const elements = [...parser.results];
  while (elements.length > 0) {
    const elem = elements.pop();
    if (elem.type == "logic" && elem.op == "and") {
      elements.push(...elem.expressions);
      continue;
    }
    if (elem.type != "expression") continue;
    if (!["sort", "limit", "ltime"].includes(elem.keyword.value)) continue;
    var obj = {};
    var start = null;
    var end = null;
    for (const [k, v] of Object.entries(elem)) {
      if (k == "type") continue;
      if (v == null) continue;
      obj[k] = v.value;
      if (start == null || start > v.col) {
        start = v.col;
      }
      if (end == null || end < v.col + v.text.length) {
        end = v.col + v.text.length;
      }
    }
    obj.start = start - 1;
    obj.len = end - start;
    result[elem.keyword.value] = obj;
  }
  return result;
}
