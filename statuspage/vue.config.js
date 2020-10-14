const GoogleFontsPlugin = require("google-fonts-webpack-plugin");

module.exports = {
    chainWebpack: config => {
        plugins: [
            new GoogleFontsPlugin({
                fonts: [
                    { family: "Material Icons" }
                ]
            })
        ]
    }
}