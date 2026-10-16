const fs = require('node:fs');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const root = path.resolve(__dirname, '..');
function run(args) {
  const result = spawnSync(process.execPath, args, {cwd: root, stdio: 'inherit'});
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status || 1);
}
for (const file of fs.readdirSync(path.join(root, 'static')).filter(f => f.endsWith('.js'))) run(['--check', 'static/' + file]);
for (const file of fs.readdirSync(path.join(root, 'tests')).filter(f => f.endsWith('.cjs'))) run(['tests/' + file]);
for (const file of fs.readdirSync(path.join(root, 'templates'))) {
  const html = fs.readFileSync(path.join(root, 'templates', file), 'utf8');
  for (const match of html.matchAll(/(?:src|href)="\/static\/([^"?]+)(?:\?[^\"]*)?"/g)) {
    if (!fs.existsSync(path.join(root, 'static', match[1]))) throw new Error(file + ': missing ' + match[1]);
  }
}
console.log('JavaScript checks, regression harnesses and template assets passed.');
