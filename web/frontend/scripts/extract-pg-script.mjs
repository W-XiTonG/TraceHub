import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const src = path.join(__dirname, '../src/views/playground/PlaygroundClassic.vue')
const content = fs.readFileSync(src, 'utf8')
const scriptMatch = content.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)
if (!scriptMatch) {
  console.error('no script found')
  process.exit(1)
}
const out = path.join(__dirname, '../src/composables/_pg_script_extract.ts')
fs.writeFileSync(out, scriptMatch[1])
console.log('extracted', scriptMatch[1].split('\n').length, 'lines to', out)
