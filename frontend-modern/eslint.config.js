import js from "@eslint/js";
import tseslint from "typescript-eslint";
import solid from "eslint-plugin-solid";
import prettier from "eslint-config-prettier";
import globals from "globals";

export default tseslint.config(
    { ignores: ["dist", "public", "node_modules", "src/api/generated"] },
    js.configs.recommended,
    ...tseslint.configs.recommended,
    {
        files: ["**/*.{ts,tsx}"],
        languageOptions: {
            globals: { ...globals.browser, ...globals.node },
            parserOptions: {
                sourceType: "module",
            },
        },
        plugins: {
            solid,
        },
        rules: {
            ...solid.configs.typescript.rules,
            "@typescript-eslint/no-unused-vars": [
                "warn",
                {
                    argsIgnorePattern: "^_",
                    varsIgnorePattern: "^_",
                    caughtErrorsIgnorePattern: "^_",
                },
            ],
            "@typescript-eslint/no-unused-expressions": "off",
            "@typescript-eslint/no-explicit-any": "off",
            "no-case-declarations": "off",
            "no-useless-escape": "off",
            "prefer-const": "off",
            "solid/reactivity": "off",
            "solid/prefer-for": "off",
            "solid/style-prop": "off",
            "solid/components-return-once": "off",
            "solid/self-closing-comp": "off",
        },
    },
    // Prevent metric threshold/color duplication in components.
    // Centralized definitions live in src/utils/metricThresholds.ts.
    {
        files: [
            "src/components/**/*.{ts,tsx}",
            "src/pages/**/*.{ts,tsx}",
        ],
        rules: {
            "no-restricted-syntax": [
                "error",
                // Metric color/threshold functions — use @/utils/metricThresholds
                {
                    selector: "FunctionDeclaration[id.name=/^get(Bar|Usage|Memory|Disk|Cpu|Metric|Threshold)(Color|Colour)/i]",
                    message: "Use getMetricColor*/getMetricSeverity from @/utils/metricThresholds instead of defining local metric color functions.",
                },
                {
                    selector: "VariableDeclarator[id.name=/^get(Bar|Usage|Memory|Disk|Cpu|Metric|Threshold)(Color|Colour)/i]",
                    message: "Use getMetricColor*/getMetricSeverity from @/utils/metricThresholds instead of defining local metric color functions.",
                },
                {
                    selector: "VariableDeclarator[id.name='METRIC_THRESHOLDS']",
                    message: "Import METRIC_THRESHOLDS from @/utils/metricThresholds instead of redefining it.",
                },
                // Time formatting — use formatRelativeTime from @/utils/format
                {
                    selector: "FunctionDeclaration[id.name=/^format(RelativeTime|TimeAgo)$/i]",
                    message: "Use formatRelativeTime from @/utils/format instead of defining local time formatting functions.",
                },
                {
                    selector: "VariableDeclarator[id.name=/^format(RelativeTime|TimeAgo)$/i]",
                    message: "Use formatRelativeTime from @/utils/format instead of defining local time formatting functions.",
                },
                // formatPowerOnHours — use @/utils/format
                {
                    selector: "FunctionDeclaration[id.name='formatPowerOnHours']",
                    message: "Use formatPowerOnHours from @/utils/format instead of defining it locally.",
                },
                {
                    selector: "VariableDeclarator[id.name='formatPowerOnHours']",
                    message: "Use formatPowerOnHours from @/utils/format instead of defining it locally.",
                },
                // estimateTextWidth — use @/utils/format
                {
                    selector: "FunctionDeclaration[id.name='estimateTextWidth']",
                    message: "Use estimateTextWidth from @/utils/format instead of defining it locally.",
                },
                {
                    selector: "VariableDeclarator[id.name='estimateTextWidth']",
                    message: "Use estimateTextWidth from @/utils/format instead of defining it locally.",
                },
                // formatAnomalyRatio / ANOMALY_SEVERITY_CLASS — use @/utils/format
                {
                    selector: "FunctionDeclaration[id.name='formatAnomalyRatio']",
                    message: "Use formatAnomalyRatio from @/utils/format instead of defining it locally.",
                },
                {
                    selector: "VariableDeclarator[id.name='formatAnomalyRatio']",
                    message: "Use formatAnomalyRatio from @/utils/format instead of defining it locally.",
                },
                {
                    selector: "VariableDeclarator[id.name='ANOMALY_SEVERITY_CLASS']",
                    message: "Import ANOMALY_SEVERITY_CLASS from @/utils/format instead of redefining it.",
                },
                // Canvas DPR setup — use setupCanvasDPR from @/utils/canvasRenderQueue
                {
                    selector: "FunctionDeclaration[id.name=/^setupCanvas(For)?DPR$/i]",
                    message: "Use setupCanvasDPR from @/utils/canvasRenderQueue instead of defining it locally.",
                },
                {
                    selector: "VariableDeclarator[id.name=/^setupCanvas(For)?DPR$/i]",
                    message: "Use setupCanvasDPR from @/utils/canvasRenderQueue instead of defining it locally.",
                },
                // Tooltip reimplementation — use useTooltip() from @/hooks/useTooltip
                {
                    selector: "CallExpression[callee.name='createSignal'] > ArrayExpression > Identifier[name=/^(showTooltip|tooltipVisible|tooltipPos|tooltipPosition)$/]",
                    message: "Use the useTooltip() hook from @/hooks/useTooltip instead of reimplementing tooltip state.",
                },
            ],
        },
    },
    prettier
);
