module.exports = {
  env: {
    browser: true,
    commonjs: true,
    es2020: true
  },
  root: true,
  // extends: ["eslint:recommended", "plugin:react/recommended", "prettier"],
  // plugins: ["prettier"],
  parserOptions: {
    ecmaVersion: 12,
    sourceType: "module"
  },
  rules: {
    // "prettier/prettier": ["off"],
    "react/react-in-jsx-scope": "off",
    "react/prop-types": "off"
  }
};
