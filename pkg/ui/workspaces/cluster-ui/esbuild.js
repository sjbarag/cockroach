const path = require("path");

const esbuild = require("esbuild");
const esbuildPluginSass = require("esbuild-plugin-sass");
const { sassPlugin, postcssModules } = require("esbuild-sass-plugin");
const stylePlugin = require("esbuild-style-plugin");
const { lessLoader } = require("esbuild-plugin-less");
const resolve = require("esbuild-plugin-resolve");
const alias = require("esbuild-plugin-alias");
const { globalExternals } = require("@fal-works/esbuild-plugin-global-externals");

const NODE_PATHS = [
  path.resolve(__dirname),
  path.resolve("./node_modules"),
  path.resolve(__dirname, "../../", "node_modules"),
  path.resolve(__dirname, "src/fonts"),
];

esbuild.build({
  nodePaths: [
    ...NODE_PATHS,
    path.resolve(__dirname, "./src/"),
  ],
  metafile: true,
  
  entryPoints: [ path.resolve(__dirname, "./src/index.ts") ],
  // outfile: path.resolve(process.env.DBCONSOLE_OUTPUT || `../../dist${process.env.DBCONSOLE_DIST}`, "assets"),
  outfile: './dist/js/main.js',
  platform: "browser",
  bundle: true,
  format: 'cjs',
  target: 'es6',
  external: [
    "react",
    "react-dom",
    "protobufjs",
    "react-router-dom",
    "react-redux",
    "redux-saga",
    "redux",
  ],
  loader: {
    ".png": "base64",
    ".jpg": "base64",
    ".gif": "base64",
    ".svg": "base64",
    ".eot": "dataurl",
    ".ttf": "dataurl",
    ".woff": "dataurl",
    ".woff2": "dataurl",
  },
  plugins: [
    resolve({
      "antd/lib/style/index": path.resolve(__dirname, "src/core/antd-patch.less"),
    }),
    alias({
      "@ant-design/icons/lib/dist": require.resolve("@ant-design/icons/lib/index.es.js"),
    }),
    // esbuildPluginSass({
    //   customSassOptions: {
    //     includePaths: [
    //       ...NODE_PATHS,
    //       path.resolve(__dirname, "./src/fonts/"),
    //     ],
    //     loadPaths: [
    //       ...NODE_PATHS,
    //       path.resolve(__dirname, "./src/fonts/"),
    //     ],
    //   },
    // }),

    // stylePlugin({
    //   renderOptions: {
    //     lessOptions: {
    //       math: "always",
    //       javascriptEnabled: true,
    //     },
    //   }
    // }),
    lessLoader({
      math: "always",
      javascriptEnabled: true,
    }),
    sassPlugin({
      transform: postcssModules({})
    }),
  ],
})
  .then((res) => {
    const fs = require("fs");
    fs.writeFileSync("esbuild.json", JSON.stringify(res.metafile));
  })
  .catch(() => process.exit(1));
