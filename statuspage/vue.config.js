const GoogleFontsPlugin = require("google-fonts-webpack-plugin");

module.exports = {
    devServer: {
        port: 8081
    },
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