// Draws each dot source it is given, as a JSON list on stdin, with the viz.js
// and the defaults a .note's page draws its diagrams with
// (assets/viz-standalone.js, assets/diagrams.js). Prints how many it drew,
// or each one viz.js refused and exits 1.
//   node scripts/draw_diagrams.cjs < sources.json
const fs = require("fs");
const path = require("path");

const repo = path.resolve(__dirname, "..");
const Viz = require(path.join(repo, "assets", "viz-standalone.js"));
const DEFAULTS = require(path.join(repo, "assets", "diagrams.js"));
const sources = JSON.parse(fs.readFileSync(0, "utf8"));

Viz.instance().then((viz) => {
  let failed = 0;
  sources.forEach((source, i) => {
    try {
      const svg = viz.renderString(source, { format: "svg", ...DEFAULTS });
      if (!svg.includes("<svg")) throw new Error("no <svg> in what viz.js drew");
    } catch (e) {
      failed += 1;
      console.log(`diagram ${i + 1}: ${e.message.split("\n")[0]}`);
    }
  });
  if (failed > 0) {
    console.log(`${failed} of ${sources.length} diagrams not drawn`);
    process.exit(1);
  }
  console.log(`${sources.length} diagrams drawn`);
}).catch((e) => {
  console.log(`viz.js did not load: ${e.message}`);
  process.exit(2);
});
