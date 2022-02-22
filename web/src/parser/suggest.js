import nearley from 'nearley'
import grammar from './query'

const queryGrammar = nearley.Grammar.fromCompiled(grammar);

export default function suggest(text, cursorOffset, groupedTags) {
    const parser = new nearley.Parser(queryGrammar);
    try {
        parser.feed(text);
    } catch (parseError) {
        console.log("Error at character " + parseError.offset); // "Error at character 9"
        return [];
    }
    // console.log(JSON.stringify(parser.results), cursorOffset);

    // Find element at cursor
    const targetElem = _findElementAtCursor(parser.results, cursorOffset);
    if (!targetElem)
        return [];
    
    const keyword = targetElem['keyword']['value'];
    const value = targetElem['value']['value'];
    if (['service', 'tag', 'mark'].includes(keyword)) {
        return groupedTags[keyword].map((t) => t.Name.split('/')[1]).filter((t) => !value || t.startsWith(value));
    }
    return [];
}

function _findElementAtCursor(results, cursorOffset) {
    const elements = [...results];
    while (elements.length > 0) {
        const elem = elements.pop();
        if (elem['type'] === 'expression') {
            if (!elem['value'])
                continue;
            
            const valueStartOffset = elem['value']['col'];
            const valueEndOffset = elem['value']['value'] ? valueStartOffset + elem['value']['value'].length : valueStartOffset;
            if (cursorOffset >= valueStartOffset && cursorOffset <= valueEndOffset)
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
