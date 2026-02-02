import js from "@eslint/js";
import tseslint from "typescript-eslint";
import solid from "eslint-plugin-solid";
import prettier from "eslint-config-prettier";
import globals from "globals";

export default tseslint.config(
    { ignores: ["dist", "public", "node_modules"] },
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
    prettier
);
