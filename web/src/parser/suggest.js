import nearley from 'nearley'
import grammar from './query'

const queryGrammar = nearley.Grammar.fromCompiled(grammar);

export default function suggest(query, cursorOffset, groupedTags, converters) {
    const parser = new nearley.Parser(queryGrammar);
    try {
        parser.feed(query);
    } catch (parseError) {
        console.log("Error at character " + parseError.offset);
        return { suggestions: [] };
    }

    // Find element at cursor
    const targetElem = _findElementAtCursor(parser.results, cursorOffset);
    if (!targetElem)
        return { suggestions: [] };

    const keyword = targetElem['keyword']['value'];
    if (['service', 'tag', 'mark', 'generated'].includes(keyword) && targetElem['value']) {
        const value = targetElem['value']['value'];
        const text = targetElem['value']['text'];
        const start = targetElem['value']['col'];
        const end = start + (text?.length ?? 0) - 1;
        const tagsInGroup = groupedTags[keyword].map((t) => t.Name.split('/')[1]);
        const suggestions = tagsInGroup.filter((t) => null == value || (t.startsWith(value) && t !== value));
        return {
            suggestions,
            start,
            end,
            type: keyword,
        };
    } else if (keyword.endsWith('data') && targetElem['converter']) {
        const value = targetElem['converter']['value'];
        const text = targetElem['converter']['text'];
        const start = targetElem['converter']['col'];
        const end = start + (text?.length ?? 0) - 1;
        const suggestions = converters.filter((c) => null == value || (c.startsWith(value) && c !== value));
        return {
            suggestions,
            start,
            end,
            type: 'data',
        };
    }
    return { suggestions: [] };
}

function _findElementAtCursor(results, cursorOffset) {
    const elements = [...results];
    const isCursorInsideElement = (elem, part) => {
        const valueStartOffset = elem[part]['col'];
        const valueEndOffset = valueStartOffset + (elem[part]['text']?.length ?? 0);
        if (cursorOffset >= valueStartOffset && cursorOffset < valueEndOffset)
            return true;
        return false;
    }
    while (elements.length > 0) {
        const elem = elements.pop();
        if (elem['type'] === 'expression') {
            if (elem['value'] && isCursorInsideElement(elem, 'value'))
                return elem;
            if (elem['converter'] && isCursorInsideElement(elem, 'converter'))
                return elem;
        }
        else if (elem['type'] === 'logic') {
            elements.push(...elem['expressions']);
        }
        else if (elem['type'] === 'not' || elem['type'] === 'subquery' || elem['type'] === 'error') {
            if (elem['expression'])
                elements.push(elem['expression']);
        }
    }
    return null;
}
