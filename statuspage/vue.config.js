module.exports = {
    devServer: {
        port: 8081
    },
    configureWebpack: {
        //nicer debugging in browser dev tools
        devtool: 'source-map'
    }
}