const colors = require("tailwindcss/colors");

module.exports = {
  purge: ["./src/**/*.{js,jsx,ts,tsx}", "./public/index.html"],
  darkMode: false, // or 'media' or 'class'
  theme: {
    extend: {
      colors: {
        "warm-gray": colors.warmGray,
        teal: colors.teal
      }
    }
  },
  variants: {
    extend: {}
  },
  plugins: [require("@tailwindcss/forms"), require("@tailwindcss/typography")]
};
