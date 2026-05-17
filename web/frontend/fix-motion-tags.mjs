import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const div = String.fromCharCode(100, 105, 118)
const root = path.join(path.dirname(fileURLToPath(import.meta.url)), 'src')

function walk(dir) {
  for (const name of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, name.name)
    if (name.isDirectory()) walk(p)
    else if (name.name.endsWith('.vue')) {
      let s = fs.readFileSync(p, 'utf8')
      if (!/<\/?motion\b/.test(s)) continue
      const next = s.replace(/<\/?motion\b/g, (m) => m.replace('motion', div))
      if (next !== s) {
        fs.writeFileSync(p, next)
        console.log(path.relative(root, p))
      }
    }
  }
}

walk(root)
