// `bun test` must use the same DOM setup and mock isolation as `bun run test`.
import { spawnSync } from 'node:child_process'

const result = spawnSync('bun', ['run', 'test'], { stdio: 'inherit' })
process.exit(result.status ?? 1)
