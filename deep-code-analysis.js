#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

class DeepCodeAnalyzer {
    constructor() {
        this.results = {
            unusedFunctions: [],
            unusedVariables: [],
            duplicateCode: [],
            commentedCode: [],
            unusedCss: [],
            unusedEventHandlers: [],
            deadConditionals: [],
            redundantFiles: [],
            unusedExports: [],
            orphanedCode: []
        };
        
        this.allFunctions = new Map();
        this.functionCalls = new Set();
        this.cssClasses = new Set();
        this.usedCssClasses = new Set();
        this.eventHandlers = new Map();
        this.domReferences = new Set();
    }
    
    analyzeFile(filePath) {
        const content = fs.readFileSync(filePath, 'utf8');
        const lines = content.split('\n');
        
        // Extract all function declarations and their locations
        const functionPatterns = [
            /function\s+(\w+)\s*\(/g,
            /const\s+(\w+)\s*=\s*function/g,
            /const\s+(\w+)\s*=\s*\([^)]*\)\s*=>/g,
            /let\s+(\w+)\s*=\s*function/g,
            /let\s+(\w+)\s*=\s*\([^)]*\)\s*=>/g,
            /var\s+(\w+)\s*=\s*function/g,
            /(\w+):\s*function\s*\(/g,
            /(\w+)\s*\([^)]*\)\s*{/g // ES6 method syntax
        ];
        
        functionPatterns.forEach(pattern => {
            let match;
            const contentCopy = content;
            while ((match = pattern.exec(contentCopy)) !== null) {
                const funcName = match[1];
                if (funcName && !['if', 'for', 'while', 'switch', 'catch', 'with'].includes(funcName)) {
                    if (!this.allFunctions.has(funcName)) {
                        this.allFunctions.set(funcName, []);
                    }
                    this.allFunctions.get(funcName).push({
                        file: filePath,
                        line: content.substring(0, match.index).split('\n').length
                    });
                }
            }
        });
        
        // Find function calls
        const callPatterns = [
            /(\w+)\s*\(/g,
            /\.(\w+)\s*\(/g,
            /\['(\w+)'\]\s*\(/g,
            /\["(\w+)"\]\s*\(/g
        ];
        
        callPatterns.forEach(pattern => {
            let match;
            const contentCopy = content;
            while ((match = pattern.exec(contentCopy)) !== null) {
                const funcName = match[1];
                if (funcName && !['if', 'for', 'while', 'switch', 'catch', 'with', 'function', 'return'].includes(funcName)) {
                    this.functionCalls.add(funcName);
                }
            }
        });
        
        // Find commented out code blocks
        const commentedCodePattern = /\/\*[\s\S]*?\*\/|\/\/.*$/gm;
        let commentMatch;
        while ((commentMatch = commentedCodePattern.exec(content)) !== null) {
            const comment = commentMatch[0];
            // Check if comment contains code-like patterns
            if (comment.match(/function|const|let|var|if|for|while|return|class|import|require/)) {
                const lineNum = content.substring(0, commentMatch.index).split('\n').length;
                this.results.commentedCode.push({
                    file: filePath,
                    line: lineNum,
                    content: comment.substring(0, 50) + '...'
                });
            }
        }
        
        // Find event handlers
        const eventHandlerPatterns = [
            /addEventListener\s*\(\s*['"](\w+)['"]/g,
            /on(\w+)\s*=/g,
            /\.on\s*\(\s*['"](\w+)['"]/g
        ];
        
        eventHandlerPatterns.forEach(pattern => {
            let match;
            while ((match = pattern.exec(content)) !== null) {
                const event = match[1];
                if (!this.eventHandlers.has(event)) {
                    this.eventHandlers.set(event, []);
                }
                this.eventHandlers.get(event).push(filePath);
            }
        });
        
        // Find DOM references
        const domPatterns = [
            /getElementById\s*\(\s*['"](\w+)['"]/g,
            /querySelector\s*\(\s*['"]#(\w+)['"]/g,
            /querySelector\s*\(\s*['"]\.(\w+)['"]/g,
            /getElementsByClassName\s*\(\s*['"](\w+)['"]/g,
            /querySelectorAll\s*\(\s*['"]\.([\w-]+)['"]/g
        ];
        
        domPatterns.forEach(pattern => {
            let match;
            while ((match = pattern.exec(content)) !== null) {
                this.domReferences.add(match[1]);
            }
        });
        
        // Find CSS class usage in JS
        const cssInJsPatterns = [
            /classList\.add\s*\(\s*['"]([^'"]+)['"]/g,
            /classList\.remove\s*\(\s*['"]([^'"]+)['"]/g,
            /classList\.toggle\s*\(\s*['"]([^'"]+)['"]/g,
            /className\s*=\s*['"]([^'"]+)['"]/g,
            /class\s*=\s*['"]([^'"]+)['"]/g
        ];
        
        cssInJsPatterns.forEach(pattern => {
            let match;
            while ((match = pattern.exec(content)) !== null) {
                const classes = match[1].split(/\s+/);
                classes.forEach(cls => {
                    if (cls) this.usedCssClasses.add(cls);
                });
            }
        });
        
        const deadConditionalPatterns = [
            /if\s*\(\s*(true|false)\s*\)/g,
            /if\s*\(\s*0\s*\)/g,
            /if\s*\(\s*1\s*\)/g,
            /while\s*\(\s*false\s*\)/g
        ];
        
        deadConditionalPatterns.forEach(pattern => {
            let match;
            while ((match = pattern.exec(content)) !== null) {
                const lineNum = content.substring(0, match.index).split('\n').length;
                this.results.deadConditionals.push({
                    file: filePath,
                    line: lineNum,
                    condition: match[0]
                });
            }
        });
    }
    
    analyzeCssFile(filePath) {
        const content = fs.readFileSync(filePath, 'utf8');
        
        // Extract CSS classes
        const classPattern = /\.([a-zA-Z][\w-]*)/g;
        let match;
        while ((match = classPattern.exec(content)) !== null) {
            this.cssClasses.add(match[1]);
        }
    }
    
    analyzeHtmlFile(filePath) {
        const content = fs.readFileSync(filePath, 'utf8');
        
        // Find class usage in HTML
        const classPattern = /class\s*=\s*["']([^"']+)["']/g;
        let match;
        while ((match = classPattern.exec(content)) !== null) {
            const classes = match[1].split(/\s+/);
            classes.forEach(cls => {
                if (cls) this.usedCssClasses.add(cls);
            });
        }
        
        // Find ID references
        const idPattern = /id\s*=\s*["'](\w+)["']/g;
        while ((match = idPattern.exec(content)) !== null) {
            this.domReferences.add(match[1]);
        }
    }
    
    findDuplicateCode() {
        const codeBlocks = new Map();
        
        // Get all JS files
        const jsFiles = execSync('find . -name "*.js" -type f | grep -v node_modules | grep -v ".min.js"', { encoding: 'utf8' })
            .trim()
            .split('\n')
            .filter(f => f);
        
        jsFiles.forEach(file => {
            const content = fs.readFileSync(file, 'utf8');
            const lines = content.split('\n');
            
            // Look for code blocks of 10+ lines
            for (let i = 0; i < lines.length - 10; i++) {
                const block = lines.slice(i, i + 10).join('\n').trim();
                if (block.length > 100) { // Meaningful code block
                    if (!codeBlocks.has(block)) {
                        codeBlocks.set(block, []);
                    }
                    codeBlocks.get(block).push({ file, line: i + 1 });
                }
            }
        });
        
        // Find duplicates
        codeBlocks.forEach((locations, block) => {
            if (locations.length > 1) {
                this.results.duplicateCode.push({
                    locations,
                    preview: block.split('\n')[0].substring(0, 50) + '...',
                    lines: block.split('\n').length
                });
            }
        });
    }
    
    async analyze() {
        console.log('🔍 Performing deep code analysis...\n');
        
        // Get all files
        const jsFiles = execSync('find . -name "*.js" -type f | grep -v node_modules | grep -v ".min.js"', { encoding: 'utf8' })
            .trim()
            .split('\n')
            .filter(f => f);
            
        const cssFiles = execSync('find . -name "*.css" -type f | grep -v node_modules | grep -v ".min.css"', { encoding: 'utf8' })
            .trim()
            .split('\n')
            .filter(f => f);
            
        const htmlFiles = execSync('find . -name "*.html" -type f | grep -v node_modules', { encoding: 'utf8' })
            .trim()
            .split('\n')
            .filter(f => f);
        
        // Analyze all files
        console.log(`Analyzing ${jsFiles.length} JS files...`);
        jsFiles.forEach(file => this.analyzeFile(file));
        
        console.log(`Analyzing ${cssFiles.length} CSS files...`);
        cssFiles.forEach(file => this.analyzeCssFile(file));
        
        console.log(`Analyzing ${htmlFiles.length} HTML files...`);
        htmlFiles.forEach(file => this.analyzeHtmlFile(file));
        
        // Find unused functions
        this.allFunctions.forEach((locations, funcName) => {
            if (!this.functionCalls.has(funcName)) {
                this.results.unusedFunctions.push({
                    name: funcName,
                    locations
                });
            }
        });
        
        // Find unused CSS classes
        this.cssClasses.forEach(cls => {
            if (!this.usedCssClasses.has(cls)) {
                this.results.unusedCss.push(cls);
            }
        });
        
        // Find duplicate code
        console.log('Looking for duplicate code blocks...');
        this.findDuplicateCode();
        
        // Generate report
        this.generateReport();
    }
    
    generateReport() {
        console.log('\n📊 DEEP CODE ANALYSIS REPORT\n');
        console.log('=' .repeat(60));
        
        // Unused functions
        console.log(`\n🔴 UNUSED FUNCTIONS (${this.results.unusedFunctions.length}):`);
        if (this.results.unusedFunctions.length > 0) {
            this.results.unusedFunctions.slice(0, 20).forEach(func => {
                console.log(`   ${func.name}:`);
                func.locations.forEach(loc => {
                    console.log(`     - ${loc.file}:${loc.line}`);
                });
            });
            if (this.results.unusedFunctions.length > 20) {
                console.log(`   ... and ${this.results.unusedFunctions.length - 20} more`);
            }
        }
        
        // Duplicate code
        console.log(`\n🔴 DUPLICATE CODE BLOCKS (${this.results.duplicateCode.length}):`);
        if (this.results.duplicateCode.length > 0) {
            this.results.duplicateCode.slice(0, 10).forEach(dup => {
                console.log(`   "${dup.preview}" (${dup.lines} lines)`);
                dup.locations.forEach(loc => {
                    console.log(`     - ${loc.file}:${loc.line}`);
                });
            });
        }
        
        // Commented code
        console.log(`\n🟡 COMMENTED OUT CODE (${this.results.commentedCode.length}):`);
        if (this.results.commentedCode.length > 0) {
            this.results.commentedCode.slice(0, 10).forEach(comment => {
                console.log(`   ${comment.file}:${comment.line} - ${comment.content}`);
            });
        }
        
        // Unused CSS
        console.log(`\n🟡 UNUSED CSS CLASSES (${this.results.unusedCss.length}):`);
        if (this.results.unusedCss.length > 0) {
            console.log(`   ${this.results.unusedCss.slice(0, 20).join(', ')}`);
            if (this.results.unusedCss.length > 20) {
                console.log(`   ... and ${this.results.unusedCss.length - 20} more`);
            }
        }
        
        // Dead conditionals
        console.log(`\n🟡 DEAD CONDITIONALS (${this.results.deadConditionals.length}):`);
        if (this.results.deadConditionals.length > 0) {
            this.results.deadConditionals.forEach(dead => {
                console.log(`   ${dead.file}:${dead.line} - ${dead.condition}`);
            });
        }
        
        // Summary
        console.log('\n' + '='.repeat(60));
        console.log('\n📈 SUMMARY:');
        const totalIssues = this.results.unusedFunctions.length + 
                          this.results.duplicateCode.length + 
                          this.results.commentedCode.length + 
                          this.results.unusedCss.length + 
                          this.results.deadConditionals.length;
        
        console.log(`   Total issues found: ${totalIssues}`);
        console.log(`   Unused functions: ${this.results.unusedFunctions.length}`);
        console.log(`   Duplicate code blocks: ${this.results.duplicateCode.length}`);
        console.log(`   Commented code blocks: ${this.results.commentedCode.length}`);
        console.log(`   Unused CSS classes: ${this.results.unusedCss.length}`);
        console.log(`   Dead conditionals: ${this.results.deadConditionals.length}`);
        
        // Estimate redundant code percentage
        const totalLines = parseInt(execSync('find . -name "*.js" -o -name "*.css" | grep -v node_modules | xargs wc -l | tail -1', { encoding: 'utf8' }).trim().split(/\s+/)[0]);
        const redundantLines = this.results.unusedFunctions.length * 10 + // estimate 10 lines per function
                             this.results.duplicateCode.length * 15 + // average duplicate block size
                             this.results.commentedCode.length * 5;   // average comment block size
        
        const redundantPercentage = ((redundantLines / totalLines) * 100).toFixed(1);
        console.log(`\n   Estimated redundant code: ~${redundantPercentage}% (${redundantLines} lines of ${totalLines} total)`);
        
        // Recommendations
        console.log('\n💡 TOP RECOMMENDATIONS:');
        console.log('   1. Remove unused functions to reduce code complexity');
        console.log('   2. Refactor duplicate code into shared utilities');
        console.log('   3. Delete commented out code (use git history instead)');
        console.log('   4. Remove unused CSS classes to reduce file size');
        console.log('   5. Clean up dead conditionals and unreachable code');
        
        // Save detailed report
        const detailedReport = {
            timestamp: new Date().toISOString(),
            summary: {
                totalIssues,
                unusedFunctions: this.results.unusedFunctions.length,
                duplicateCode: this.results.duplicateCode.length,
                commentedCode: this.results.commentedCode.length,
                unusedCss: this.results.unusedCss.length,
                deadConditionals: this.results.deadConditionals.length,
                estimatedRedundancy: `${redundantPercentage}%`
            },
            details: this.results
        };
        
        fs.writeFileSync('dead-code-report.json', JSON.stringify(detailedReport, null, 2));
        console.log('\n📁 Detailed report saved to: dead-code-report.json');
    }
}

// Run the analyzer
const analyzer = new DeepCodeAnalyzer();
analyzer.analyze().catch(console.error);