const path = require("path");
const esbuild = require("esbuild");
const { stylusLoader } = require("esbuild-stylus-loader");
const nib = require("nib");

const DISTNAME = process.env.CRDB_OSS ? "oss" : "ccl";

esbuild.build({
  entryPoints: [
    "./src/index.tsx",
  ],
  outfile: path.normalize(
    path.join(__dirname, "..", "..", `dist${DISTNAME}`, "assets", "bundle.js")
  ),
  platform: "browser",
  target: "es6",
  bundle: true,
  minify: true,
  sourcemap: true,
  loader: {
    ".png": "base64",
    ".jpg": "base64",
    ".gif": "base64",
    ".svg": "base64",
    ".eot": "dataurl",
    ".woff": "dataurl",
    ".woff2": "dataurl",
  },
  external: [
    // HACK(barag): ignore assets loaded webpack-specific loader syntax for now
    "!!raw-loader*",
    "!!url-loader*",
  ],
  plugins: [
    stylusLoader({
      stylusOptions: {
        include: [
          __dirname,
          path.normalize(path.join(__dirname, "..", "..", "node_modules")),
        ],
        use: [
          (stylus) => stylus.use(nib()),
        ],
      }
    }),
  ],
}).catch(err => process.exit(err.errors.length));
