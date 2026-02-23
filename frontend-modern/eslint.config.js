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
                // Block hardcoded legacy colors in favor of semantic design system.
                // Only flags shades (50-200, 700-800) that look wrong in the opposite
                // color mode. Mid-range grays (300-600) are mode-neutral. 900/950 are
                // reserved for intentional terminal/code UIs.
                // dark:/hover:/focus: variants are excluded by the (?:^|\s) anchor.
                {
                    selector: "Literal[value=/(?:^|\\s)(?:bg|text|border|ring)-(?:slate|gray|zinc|neutral)-(?:50|100|200|700|800)(?:$|\\s)/]",
                    message: "Use semantic design system classes (e.g. bg-surface, text-base-content, border-border) instead of hardcoded tailwind grays. See DESIGN_SYSTEM.md for the token reference.",
                },
                {
                    selector: "TemplateElement[value.raw=/(?:^|\\s)(?:bg|text|border|ring)-(?:slate|gray|zinc|neutral)-(?:50|100|200|700|800)(?:$|\\s)/]",
                    message: "Use semantic design system classes (e.g. bg-surface, text-base-content, border-border) instead of hardcoded tailwind grays. See DESIGN_SYSTEM.md for the token reference.",
                },
                // Full-viewport shells must use semantic background tokens.
                // Hardcoded Tailwind color scales on min-h-screen wrappers are a recurring
                // source of light/dark mismatches on auth/loading screens.
                {
                    selector: "Literal[value=/(?=.*(?:^|\\s)min-h-screen(?:$|\\s))(?=.*(?:^|\\s)bg-(?:white|black|slate-\\d{2,3}|gray-\\d{2,3}|zinc-\\d{2,3}|neutral-\\d{2,3}|stone-\\d{2,3}|red-\\d{2,3}|orange-\\d{2,3}|amber-\\d{2,3}|yellow-\\d{2,3}|lime-\\d{2,3}|green-\\d{2,3}|emerald-\\d{2,3}|teal-\\d{2,3}|cyan-\\d{2,3}|sky-\\d{2,3}|blue-\\d{2,3}|indigo-\\d{2,3}|violet-\\d{2,3}|purple-\\d{2,3}|fuchsia-\\d{2,3}|pink-\\d{2,3}|rose-\\d{2,3})(?:$|\\s))/]",
                    message: "Full-screen wrappers (min-h-screen) must use semantic backgrounds (bg-base/bg-surface/bg-surface-alt) instead of hardcoded palette classes.",
                },
                {
                    selector: "TemplateElement[value.raw=/(?=.*(?:^|\\s)min-h-screen(?:$|\\s))(?=.*(?:^|\\s)bg-(?:white|black|slate-\\d{2,3}|gray-\\d{2,3}|zinc-\\d{2,3}|neutral-\\d{2,3}|stone-\\d{2,3}|red-\\d{2,3}|orange-\\d{2,3}|amber-\\d{2,3}|yellow-\\d{2,3}|lime-\\d{2,3}|green-\\d{2,3}|emerald-\\d{2,3}|teal-\\d{2,3}|cyan-\\d{2,3}|sky-\\d{2,3}|blue-\\d{2,3}|indigo-\\d{2,3}|violet-\\d{2,3}|purple-\\d{2,3}|fuchsia-\\d{2,3}|pink-\\d{2,3}|rose-\\d{2,3})(?:$|\\s))/]",
                    message: "Full-screen wrappers (min-h-screen) must use semantic backgrounds (bg-base/bg-surface/bg-surface-alt) instead of hardcoded palette classes.",
                },
                // Catch orphaned CSS prefixes (e.g. "hover: " with no utility after it).
                // These produce no CSS output and are always a bug from bad find-replace.
                {
                    selector: "Literal[value=/(?:^|\\s)(?:hover|focus|active|dark|group-hover|peer-focus|before|after|lg|md|sm|xl|2xl):\\s/]",
                    message: "Orphaned CSS prefix detected (e.g. 'hover: ' with nothing after it). This is a broken class string — the utility class is missing.",
                },
                {
                    selector: "TemplateElement[value.raw=/(?:^|\\s)(?:hover|focus|active|dark|group-hover|peer-focus|before|after|lg|md|sm|xl|2xl):\\s/]",
                    message: "Orphaned CSS prefix detected (e.g. 'hover: ' with nothing after it). This is a broken class string — the utility class is missing.",
                },
                // Block dark-mode white backgrounds, which are almost always unreadable.
                {
                    selector: "Literal[value=/(?:^|\\s)dark:(?:hover:)?bg-white(?:$|\\s)/]",
                    message: "Avoid dark:bg-white / dark:hover:bg-white. Use semantic surface tokens for dark mode-safe contrast.",
                },
                {
                    selector: "TemplateElement[value.raw=/(?:^|\\s)dark:(?:hover:)?bg-white(?:$|\\s)/]",
                    message: "Avoid dark:bg-white / dark:hover:bg-white. Use semantic surface tokens for dark mode-safe contrast.",
                },
                // Prevent low-contrast combinations that caused prior regressions.
                {
                    selector: "Literal[value=/(?=.*(?:^|\\s)bg-base(?:$|\\s))(?=.*(?:^|\\s)text-white(?:$|\\s))/]",
                    message: "Avoid combining bg-base with text-white. Use semantic pairs like bg-surface + text-base-content or a dedicated accent background.",
                },
                {
                    selector: "TemplateElement[value.raw=/(?=.*(?:^|\\s)bg-base(?:$|\\s))(?=.*(?:^|\\s)text-white(?:$|\\s))/]",
                    message: "Avoid combining bg-base with text-white. Use semantic pairs like bg-surface + text-base-content or a dedicated accent background.",
                },
            ],
        },
    },
    {
        files: [
            "src/components/Settings/DiagnosticsPanel.tsx",
            "src/components/Settings/ReportingPanel.tsx",
            "src/components/Settings/SystemLogsPanel.tsx",
        ],
        rules: {
            "no-restricted-imports": [
                "error",
                {
                    paths: [
                        {
                            name: "@/components/shared/SettingsPanel",
                            message: "Use @/components/Settings/OperationsPanel so Operations tab headers remain consistent.",
                        },
                    ],
                },
            ],
        },
    },
    prettier
);
