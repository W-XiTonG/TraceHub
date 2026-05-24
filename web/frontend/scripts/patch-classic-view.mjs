import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const core = fs.readFileSync(path.join(__dirname, '../src/composables/usePlaygroundCore.ts'), 'utf8')
const match = core.match(/\n  return \{\n([\s\S]*?)\n  \}\n\}\s*$/)
if (!match) {
  console.error('return block not found')
  process.exit(1)
}
const keys = match[1]
  .split(',')
  .map((s) => s.trim())
  .filter(Boolean)

const script = `<script setup lang="ts">
import ThLoadingSpinner from '../../components/ui/ThLoadingSpinner.vue'
import { RouterLink } from 'vue-router'
import { usePlaygroundCore } from '../../composables/usePlaygroundCore'

const {
  ${keys.join(',\n  ')},
} = usePlaygroundCore({ modernUi: false })
</script>`

const classicPath = path.join(__dirname, '../src/views/playground/PlaygroundClassic.vue')
const content = fs.readFileSync(classicPath, 'utf8')
const updated = content.replace(/<script setup lang="ts">[\s\S]*?<\/script>/, script)
fs.writeFileSync(classicPath, updated)
console.log('updated PlaygroundClassic.vue with', keys.length, 'bindings')
