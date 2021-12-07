const path = require("path");

const esbuild = require("esbuild");
const copy = require("esbuild-plugin-copy").default;
const inlineImage = require("esbuild-plugin-inline-image");
const stylePlugin = require("esbuild-style-plugin");
const { stylusLoader } = require("esbuild-stylus-loader");
const sassPlugin = require("esbuild-plugin-sass");
const { lessLoader } = require("esbuild-plugin-less");
const alias = require("esbuild-plugin-alias");
const nib = require("nib")();

const NODE_PATHS = [
  path.resolve(__dirname),
  path.resolve("./node_modules"),
  path.resolve(__dirname, "../../", "node_modules"),
  path.resolve(__dirname, "./fonts"),
];

esbuild.build({
  //logLevel: "debug",
  nodePaths: NODE_PATHS,
  metafile: true,
  minify: true,
  preserveSymlinks: false,
  
  entryPoints: [ path.resolve(__dirname, "./src/index.tsx") ],
  // outfile: path.resolve(process.env.DBCONSOLE_OUTPUT || `../../dist${process.env.DBCONSOLE_DIST}`, "assets"),
  outfile: '../../distccl/assets/bundle.js',
  bundle: true,
  format: 'iife',
  target: 'es6',
  sourcemap: true,
  inject: [
    "./src/util/buffer-shim.ts",
  ],
  define: {
    "process.versions.node": JSON.stringify(process.versions.node),
  },
  loader: {
    ".eot": "dataurl",
    ".ttf": "dataurl",
    ".woff": "dataurl",
    ".woff2": "dataurl",
  },
  plugins: [
    copy({ from: "./favicon.ico", to: "favicon.ico" }),
    alias({
      "@ant-design/icons/lib/dist": require.resolve("@ant-design/icons/lib/index.es.js"),
    }),
    inlineImage({
      extensions: [ "png", "jpg", "gif", "svg", ],
      limit: 10000,
    }),
    stylusLoader({
      stylusOptions: {
        include: [
          ...NODE_PATHS,
          path.resolve(__dirname, "./src/"),
          path.resolve(__dirname, "./styl/"),
          path.resolve(__dirname, "./"),
        ],
        use: [
          (stylus) => stylus.use(nib),
        ],
      }
    }),
    sassPlugin({
      paths: [
        ...NODE_PATHS
      ],
    }),
    lessLoader({
      math: "always",
      javascriptEnabled: true,
    }),

    // stylePlugin({
    //   extract: false,
    //   renderOptions: {
    //     stylusOptions: {
    //       paths: [
    //         path.resolve(__dirname),
    //         path.resolve(__dirname, "./src"),
    //         path.resolve(__dirname, "./styl"),
    //         path.resolve(__dirname, "./node_modules/"),
    //         path.resolve(__dirname, "../../node_modules/"),
    //       ],
    //       use: nib,
    //     },
    //     sassOptions: {
    //       paths: [
    //         ...NODE_PATHS
    //       ],
    //     },
    //     lessOptions: {
    //       math: "always",
    //       javascriptEnabled: true,
    //     },
    //   },
    // }),
  ],
})
  .then((res) => {
    const fs = require("fs");
    fs.writeFileSync("esbuild.json", JSON.stringify(res.metafile));
  })
  .catch(() => process.exit(1));
