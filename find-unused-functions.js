#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

// Store all function definitions and their usage
const functions = new Map();
const functionCalls = new Set();
const exportedFunctions = new Set();
const methodCalls = new Set();

// Regex patterns for finding functions
const patterns = {
  // Function declarations: function name() {}
  functionDeclaration: /function\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\(/g,
  
  // Function expressions: const name = function() {}
  functionExpression: /(?:const|let|var)\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*=\s*function\s*\(/g,
  
  // Arrow functions: const name = () => {}
  arrowFunction: /(?:const|let|var)\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*=\s*(?:\([^)]*\)|[a-zA-Z_$][a-zA-Z0-9_$]*)\s*=>/g,
  
  // Method definitions in objects: name() {} or name: function() {}
  methodDefinition: /([a-zA-Z_$][a-zA-Z0-9_$]*)\s*(?:\([^)]*\)\s*{|:\s*function\s*\()/g,
  
  // Class methods: name() {}
  classMethod: /(?:static\s+)?([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\([^)]*\)\s*{/g,
  
  // Function calls: name() or object.name()
  functionCall: /(?:^|[^a-zA-Z0-9_$])([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\(/g,
  
  // Method calls: object.method() or this.method()
  methodCall: /\.\s*([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\(/g,
  
  // Exports
  exports: /(?:export\s+(?:default\s+)?(?:function\s+)?|exports\.|module\.exports\s*[=.]\s*)([a-zA-Z_$][a-zA-Z0-9_$]*)/g,
  
  // Event handlers in HTML strings
  eventHandler: /(?:on[a-zA-Z]+)\s*=\s*["']([a-zA-Z_$][a-zA-Z0-9_$]*)\(/g,
  
  // Dynamic function calls
  dynamicCall: /\[["']([a-zA-Z_$][a-zA-Z0-9_$]*)["']\]\s*\(/g
};

// List of common false positives to ignore
const ignoredFunctions = new Set([
  'constructor', 'render', 'getElementById', 'querySelector', 'querySelectorAll',
  'addEventListener', 'removeEventListener', 'setTimeout', 'setInterval',
  'clearTimeout', 'clearInterval', 'fetch', 'Promise', 'console',
  'parseInt', 'parseFloat', 'toString', 'valueOf', 'JSON', 'Math',
  'Object', 'Array', 'String', 'Number', 'Boolean', 'Date', 'RegExp',
  'require', 'exports', 'module', 'process', 'Buffer', '__dirname', '__filename'
]);

function findJavaScriptFiles(dir) {
  const files = [];
  
  function walk(currentDir) {
    try {
      const items = fs.readdirSync(currentDir);
      
      for (const item of items) {
        const fullPath = path.join(currentDir, item);
        const stat = fs.statSync(fullPath);
        
        if (stat.isDirectory() && !item.startsWith('.') && item !== 'node_modules') {
          walk(fullPath);
        } else if (stat.isFile() && item.endsWith('.js')) {
          files.push(fullPath);
        }
      }
    } catch (err) {
      // Skip directories we can't read
    }
  }
  
  walk(dir);
  return files;
}

function analyzeFile(filePath) {
  try {
    const content = fs.readFileSync(filePath, 'utf8');
    const relativePath = path.relative('/opt/pulse', filePath);
    
    // Skip minified files
    if (filePath.includes('.min.js') || content.length > 100000) {
      return;
    }
    
    // Find function definitions
    let match;
    
    // Function declarations
    patterns.functionDeclaration.lastIndex = 0;
    while ((match = patterns.functionDeclaration.exec(content)) !== null) {
      const funcName = match[1];
      if (!ignoredFunctions.has(funcName)) {
        const lineNum = content.substring(0, match.index).split('\n').length;
        if (!functions.has(funcName)) {
          functions.set(funcName, []);
        }
        functions.get(funcName).push({
          file: relativePath,
          line: lineNum,
          type: 'function declaration'
        });
      }
    }
    
    // Function expressions
    patterns.functionExpression.lastIndex = 0;
    while ((match = patterns.functionExpression.exec(content)) !== null) {
      const funcName = match[1];
      if (!ignoredFunctions.has(funcName)) {
        const lineNum = content.substring(0, match.index).split('\n').length;
        if (!functions.has(funcName)) {
          functions.set(funcName, []);
        }
        functions.get(funcName).push({
          file: relativePath,
          line: lineNum,
          type: 'function expression'
        });
      }
    }
    
    // Arrow functions
    patterns.arrowFunction.lastIndex = 0;
    while ((match = patterns.arrowFunction.exec(content)) !== null) {
      const funcName = match[1];
      if (!ignoredFunctions.has(funcName)) {
        const lineNum = content.substring(0, match.index).split('\n').length;
        if (!functions.has(funcName)) {
          functions.set(funcName, []);
        }
        functions.get(funcName).push({
          file: relativePath,
          line: lineNum,
          type: 'arrow function'
        });
      }
    }
    
    // Class methods (special handling to avoid constructor)
    const classMatches = content.match(/class\s+[a-zA-Z_$][a-zA-Z0-9_$]*(?:\s+extends\s+[a-zA-Z_$][a-zA-Z0-9_$]*)?\s*{[^}]+}/g) || [];
    for (const classBody of classMatches) {
      patterns.classMethod.lastIndex = 0;
      while ((match = patterns.classMethod.exec(classBody)) !== null) {
        const methodName = match[1];
        if (!ignoredFunctions.has(methodName) && methodName !== 'constructor') {
          const lineNum = content.substring(0, content.indexOf(classBody) + match.index).split('\n').length;
          if (!functions.has(methodName)) {
            functions.set(methodName, []);
          }
          functions.get(methodName).push({
            file: relativePath,
            line: lineNum,
            type: 'class method'
          });
        }
      }
    }
    
    // Find function calls
    patterns.functionCall.lastIndex = 0;
    while ((match = patterns.functionCall.exec(content)) !== null) {
      const funcName = match[1];
      if (!ignoredFunctions.has(funcName)) {
        functionCalls.add(funcName);
      }
    }
    
    // Find method calls
    patterns.methodCall.lastIndex = 0;
    while ((match = patterns.methodCall.exec(content)) !== null) {
      const methodName = match[1];
      if (!ignoredFunctions.has(methodName)) {
        methodCalls.add(methodName);
      }
    }
    
    // Find exports
    patterns.exports.lastIndex = 0;
    while ((match = patterns.exports.exec(content)) !== null) {
      const exportName = match[1];
      if (!ignoredFunctions.has(exportName)) {
        exportedFunctions.add(exportName);
      }
    }
    
    // Find event handlers in HTML strings
    patterns.eventHandler.lastIndex = 0;
    while ((match = patterns.eventHandler.exec(content)) !== null) {
      const handlerName = match[1];
      if (!ignoredFunctions.has(handlerName)) {
        functionCalls.add(handlerName);
      }
    }
    
    // Find dynamic calls
    patterns.dynamicCall.lastIndex = 0;
    while ((match = patterns.dynamicCall.exec(content)) !== null) {
      const funcName = match[1];
      if (!ignoredFunctions.has(funcName)) {
        functionCalls.add(funcName);
      }
    }
    
  } catch (err) {
    console.error(`Error analyzing ${filePath}:`, err.message);
  }
}

// Main analysis
console.log('Analyzing JavaScript files for unused functions...\n');

// Find all JS files in src/public/js and server directories
const publicJsFiles = findJavaScriptFiles('/opt/pulse/src/public/js');
const serverFiles = findJavaScriptFiles('/opt/pulse/server');
const allFiles = [...publicJsFiles, ...serverFiles];

console.log(`Found ${allFiles.length} JavaScript files to analyze\n`);

// Analyze each file
for (const file of allFiles) {
  analyzeFile(file);
}

// Combine all calls
const allCalls = new Set([...functionCalls, ...methodCalls]);

// Find unused functions
const unusedFunctions = [];
for (const [funcName, locations] of functions.entries()) {
  if (!allCalls.has(funcName) && !exportedFunctions.has(funcName)) {
    unusedFunctions.push({ name: funcName, locations });
  }
}

// Sort by file path
unusedFunctions.sort((a, b) => {
  const fileA = a.locations[0].file;
  const fileB = b.locations[0].file;
  return fileA.localeCompare(fileB);
});

// Output results
console.log('=== POTENTIALLY UNUSED FUNCTIONS ===\n');

if (unusedFunctions.length === 0) {
  console.log('No unused functions found!');
} else {
  let currentFile = '';
  for (const func of unusedFunctions) {
    for (const loc of func.locations) {
      if (loc.file !== currentFile) {
        currentFile = loc.file;
        console.log(`\n## ${currentFile}`);
      }
      console.log(`  - ${func.name} (line ${loc.line}) - ${loc.type}`);
    }
  }
  
  console.log(`\n\nTotal potentially unused functions: ${unusedFunctions.length}`);
  console.log('\nNote: Some of these might be:');
  console.log('- Called dynamically or through eval()');
  console.log('- Used in HTML files as event handlers');
  console.log('- Exported for external use');
  console.log('- Called from other files not analyzed');
  console.log('\nAlways verify before removing!');
}