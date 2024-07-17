import nearley from "nearley";
import grammar, {
  ExpressionQueryElement,
  QueryElement,
  isExpression,
  isLogicExpression,
  isSubExpression,
} from "./query";
import { ConverterStatistics, TagInfo } from "@/apiClient";

type SuggestionResults = {
  suggestions: string[];
  start: number;
  end: number;
  type: string;
};

const queryGrammar = nearley.Grammar.fromCompiled(grammar);

export default function suggest(
  query: string,
  cursorOffset: number,
  groupedTags: { [key: string]: TagInfo[] },
  converters: ConverterStatistics[] | null
): SuggestionResults {
  const parser = new nearley.Parser(queryGrammar);
  try {
    parser.feed(query);
  } catch (parseError) {
    const offset = (parseError as { offset: number }).offset;
    console.log(`Error at character ${offset}`);
    return { suggestions: [], start: 0, end: 0, type: "tag" };
  }

  // Find element at cursor
  const targetElem = _findElementAtCursor(
    parser.results as QueryElement[],
    cursorOffset
  );
  if (!targetElem) return { suggestions: [], start: 0, end: 0, type: "tag" };

  const keyword = targetElem.keyword.value;
  if (
    ["service", "tag", "mark", "generated"].includes(keyword) &&
    targetElem.value !== undefined
  ) {
    const value = targetElem.value.value;
    const text = targetElem.value.text;
    const start = targetElem.value.col;
    const end = start + (text.length ?? 0) - 1;
    const tagsInGroup = groupedTags[keyword].map((t) => t.Name.split("/")[1]);
    const suggestions = tagsInGroup.filter(
      (t) => t.startsWith(value) && t !== value
    );
    return {
      suggestions,
      start,
      end,
      type: keyword,
    };
  } else if (
    keyword.endsWith("data") &&
    targetElem.converter != null &&
    converters !== null
  ) {
    const value = targetElem.converter.value;
    const text = targetElem.converter.text;
    const start = targetElem.converter.col;
    const end = start + (text.length ?? 0) - 1;
    const suggestions = converters
      .filter((c) => c.Name.startsWith(value) && c.Name !== value)
      .map((c) => c.Name);
    return {
      suggestions,
      start,
      end,
      type: "data",
    };
  }
  return { suggestions: [], start: 0, end: 0, type: "tag" };
}

function _findElementAtCursor(
  results: QueryElement[],
  cursorOffset: number
): ExpressionQueryElement | null {
  const elements = [...results];
  const isCursorInsideElement = (
    elem: ExpressionQueryElement,
    part: "value" | "converter"
  ) => {
    const partValue = elem[part];
    if (!partValue) return false;
    const valueStartOffset = partValue.col;
    const valueEndOffset = valueStartOffset + (partValue.text.length ?? 0);
    if (cursorOffset >= valueStartOffset && cursorOffset < valueEndOffset)
      return true;
    return false;
  };
  while (elements.length > 0) {
    const elem = elements.pop();
    if (elem === undefined) break;
    if (isExpression(elem)) {
      if (elem.value !== undefined && isCursorInsideElement(elem, "value"))
        return elem;
      if (
        elem.converter !== undefined &&
        isCursorInsideElement(elem, "converter")
      )
        return elem;
    } else if (isLogicExpression(elem)) {
      elements.push(...elem.expressions);
    } else if (isSubExpression(elem)) {
      if (elem.expression) elements.push(elem.expression);
    }
  }
  return null;
}
