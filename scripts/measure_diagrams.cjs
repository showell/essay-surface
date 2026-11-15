// Draws each dot source it is given, as a JSON list on stdin, with the viz.js
// and the defaults a .note's page draws its diagrams with
// (assets/viz-standalone.js, assets/diagrams.js), as draw_diagrams.cjs does,
// and prints what each drawing measures as a JSON list, one entry a source:
// {drawn, width_px, height_px, nodes, edges, clusters, longest_label,
// longest_label_text}, or {drawn: false, error} for one viz.js refused.
// check_note.shale judges the numbers; this only measures.
//
// The size is the SVG's own, the size a browser draws it at before the
// page's CSS scales it: Graphviz writes it in points, and a CSS pixel is
// 3/4 of a point, so a pixel count is the points times 4/3, rounded up. The
// counts are of what was drawn: the SVG's node, edge and cluster groups. An
// edge's label is every text the edge's group draws (its label, head label
// and tail label, each line of each), joined by a space, so it is the
// number of characters a reader reads along that edge.
//   node scripts/measure_diagrams.cjs < sources.json
const fs = require("fs");
const path = require("path");

const repo = path.resolve(__dirname, "..");
const Viz = require(path.join(repo, "assets", "viz-standalone.js"));
const DEFAULTS = require(path.join(repo, "assets", "diagrams.js"));
const sources = JSON.parse(fs.readFileSync(0, "utf8"));

function unescaped(s) {
  return s
    .replace(/&#(\d+);/g, (_, n) => String.fromCodePoint(Number(n)))
    .replace(/&#x([0-9a-fA-F]+);/g, (_, n) => String.fromCodePoint(parseInt(n, 16)))
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&apos;/g, "'")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&");
}

// The groups Graphviz draws, by their class, each group's text up to the
// next group's start.
function groups(svg, kind) {
  const out = [];
  const parts = svg.split(/<g id="[^"]*" class="/);
  for (const part of parts.slice(1)) {
    if (part.startsWith(kind + '"')) out.push(part);
  }
  return out;
}

function points(svg, attr) {
  const m = svg.match(new RegExp("<svg[^>]*\\s" + attr + '="([0-9.]+)pt"'));
  if (!m) throw new Error("the SVG has no " + attr + " in points");
  return Number(m[1]);
}

function measure(svg) {
  let longest = "";
  for (const edge of groups(svg, "edge")) {
    const texts = [...edge.matchAll(/<text[^>]*>([^<]*)<\/text>/g)].map((m) => unescaped(m[1]));
    const label = texts.join(" ");
    if ([...label].length > [...longest].length) longest = label;
  }
  return {
    drawn: true,
    width_px: Math.ceil((points(svg, "width") * 4) / 3),
    height_px: Math.ceil((points(svg, "height") * 4) / 3),
    nodes: groups(svg, "node").length,
    edges: groups(svg, "edge").length,
    clusters: groups(svg, "cluster").length,
    longest_label: [...longest].length,
    longest_label_text: longest,
  };
}

Viz.instance().then((viz) => {
  const out = sources.map((source) => {
    try {
      const svg = viz.renderString(source, { format: "svg", ...DEFAULTS });
      if (!svg.includes("<svg")) throw new Error("no <svg> in what viz.js drew");
      return measure(svg);
    } catch (e) {
      return { drawn: false, error: e.message.split("\n")[0] };
    }
  });
  console.log(JSON.stringify(out));
}).catch((e) => {
  console.log(`viz.js did not load: ${e.message}`);
  process.exit(2);
});
