// craco.config.js
module.exports = {
  style: {
    postcss: {
      // eslint-disable-next-line global-require, import/no-extraneous-dependencies
      plugins: [require("tailwindcss"), require("autoprefixer")],
    },
  },
};
