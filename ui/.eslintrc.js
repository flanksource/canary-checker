module.exports = {
  env: {
    browser: true,
    commonjs: true,
    es2020: true,
    "jest/globals": true,
  },
  settings: {
    jest: {
      version: "27",
    },
  },
  ignorePatterns: ["node_modules", "build"],
  root: true,
  extends: [
    "eslint:recommended",
    "airbnb",
    "plugin:react/recommended",
    "plugin:jsx-a11y/recommended",
    "plugin:jest/recommended",
    "plugin:react-hooks/recommended",
    "prettier",
  ],
  plugins: ["prettier"],
  parserOptions: {
    ecmaVersion: 12,
    sourceType: "module",
    jsx: true,
    ecmaFeatures: {
      impliedStrict: true,
    },
  },
  rules: {
    "prettier/prettier": ["error"],
    "react/react-in-jsx-scope": "off",
    "react/prop-types": "off",
    "react/jsx-filename-extension": "off",
    "react/jsx-props-no-spreading": 0,
    "no-use-before-define": ["error", { functions: false, classes: true }],
    "no-shadow": "off",
    "no-param-reassign": "off",
  },
};
