#!/usr/bin/env node
/**
 * JSON 处理工具主入口
 */

function formatJson(input, indent = 2) {
    try {
        const parsed = JSON.parse(input);
        const formatted = JSON.stringify(parsed, null, indent);
        return { output: formatted, is_error: false };
    } catch (e) {
        return { output: `Invalid JSON: ${e.message}`, is_error: true };
    }
}

function validateJson(input) {
    try {
        JSON.parse(input);
        return { output: "Valid JSON", is_error: false };
    } catch (e) {
        return { output: `Invalid JSON: ${e.message}`, is_error: true };
    }
}

function main() {
    const args = process.argv.slice(2);
    
    if (args.length < 1) {
        console.log(JSON.stringify({
            output: "Usage: node index.js <tool_name> [params_json]",
            is_error: true
        }));
        return;
    }

    const toolName = args[0];
    let params = {};

    if (args.length > 1) {
        try {
            params = JSON.parse(args[1]);
        } catch (e) {
            console.log(JSON.stringify({
                output: "Invalid JSON params",
                is_error: true
            }));
            return;
        }
    }

    let result;
    if (toolName === "format_json") {
        result = formatJson(params.input, params.indent || 2);
    } else if (toolName === "validate_json") {
        result = validateJson(params.input);
    } else {
        result = { output: `Unknown tool: ${toolName}`, is_error: true };
    }

    console.log(JSON.stringify(result));
}

if (require.main === module) {
    main();
}
