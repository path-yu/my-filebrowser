import fs from "node:fs";
const s = fs.readFileSync(
  "node_modules/ace-builds/src-noconflict/ext-themelist.js",
  "utf8"
);
// entries look like: ["Caption"], ["Caption", "name", "light"|"dark"]
const re = /\[\s*"((?:[^"\\]|\\.)*)"\s*(?:,\s*"((?:[^"\\]|\\.)*)"\s*(?:,\s*"(light|dark)"\s*)?)?\]/g;
const names = new Set();
let m;
while ((m = re.exec(s))) {
  const name = m[2] || m[1].toLowerCase().replace(/\s+/g, "_");
  names.add(name);
}
const sorted = [...names].sort();
console.log(JSON.stringify(sorted));
console.error("count=" + sorted.length);
