const nearley = require("nearley");
const grammar = require("./query.js");

const parser = new nearley.Parser(nearley.Grammar.fromCompiled(grammar));
try {
    // parser.feed(`service: service:asd`);
    parser.feed(`@wat:service:asd or ( tag:"h\ni" and id:444 )`);
    // parser.feed(`service:"asd" or -service:abc sort:`);
} catch (parseError) {
    console.log("Error at character " + parseError.offset); // "Error at character 9"
}

console.log(JSON.stringify(parser.results));