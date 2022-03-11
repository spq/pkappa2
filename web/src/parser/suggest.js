import nearley from 'nearley'
import grammar from './query'

const queryGrammar = nearley.Grammar.fromCompiled(grammar);

export default function suggest(query, cursorOffset, groupedTags) {
    const parser = new nearley.Parser(queryGrammar);
    try {
        parser.feed(query);
    } catch (parseError) {
        console.log("Error at character " + parseError.offset); // "Error at character 9"
        return {suggestions: []};
    }
    console.log(JSON.stringify(parser.results), cursorOffset);

    // Find element at cursor
    const targetElem = _findElementAtCursor(parser.results, cursorOffset);
    if (!targetElem)
        return {suggestions: []};
    
    const keyword = targetElem['keyword']['value'];
    const value = targetElem['value']['value'];
    const text = targetElem['value']['text'];
    if (['service', 'tag', 'mark'].includes(keyword)) {
        const start = targetElem['value']['col']; 
        const end = start + (text?.length ?? 0);
        const tagsInGroup = groupedTags[keyword].map((t) => t.Name.split('/')[1]);
        const suggestions = tagsInGroup.filter((t) => null == value || (t.startsWith(value) && t !== value));
        return {
            suggestions,
            start,
            end,
        };
    }
    return {suggestions: []};
}

function _findElementAtCursor(results, cursorOffset) {
    const elements = [...results];
    while (elements.length > 0) {
        const elem = elements.pop();
        if (elem['type'] === 'expression') {
            if (!elem['value'])
                continue;
            
            const valueStartOffset = elem['value']['col'];
            const valueEndOffset = valueStartOffset + (elem['value']['text']?.length ?? 0);
            if (cursorOffset >= valueStartOffset && cursorOffset < valueEndOffset)
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
