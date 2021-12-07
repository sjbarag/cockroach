const esbuild = require("esbuild");

esbuild.build({
  metafile: true,
  entryPoints: [
    "./src/js/protos.js",
  ],
  define: {
    global: "window",
    'process.env.NODE_ENV': JSON.stringify("production"),
  },
  bundle: true,
  outfile: '../../distccl/assets/protos.js',
  external: [
    'react',
  ],
}).then((res) => {
  const fs = require("fs");
  fs.writeFileSync("esbuild.protos.json", JSON.stringify(res.metafile));
}).catch(() => process.exit(1));
