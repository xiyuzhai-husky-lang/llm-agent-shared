import runtimeWasm from 'web-tree-sitter/web-tree-sitter.wasm?url'
import type { GrammarLanguage } from './languages'
import { createTreeSitterHighlighter } from './highlight'

const localGrammars = {
  latex: () => import('./wasm/tree-sitter-latex.wasm?url'),
}

const grammars = {
  ...localGrammars,
  bash: () => import('tree-sitter-wasms/out/tree-sitter-bash.wasm?url'),
  c: () => import('tree-sitter-wasms/out/tree-sitter-c.wasm?url'),
  c_sharp: () => import('tree-sitter-wasms/out/tree-sitter-c_sharp.wasm?url'),
  cpp: () => import('tree-sitter-wasms/out/tree-sitter-cpp.wasm?url'),
  css: () => import('tree-sitter-wasms/out/tree-sitter-css.wasm?url'),
  dart: () => import('tree-sitter-wasms/out/tree-sitter-dart.wasm?url'),
  elisp: () => import('tree-sitter-wasms/out/tree-sitter-elisp.wasm?url'),
  elixir: () => import('tree-sitter-wasms/out/tree-sitter-elixir.wasm?url'),
  embedded_template: () => import('tree-sitter-wasms/out/tree-sitter-embedded_template.wasm?url'),
  go: () => import('tree-sitter-wasms/out/tree-sitter-go.wasm?url'),
  html: () => import('tree-sitter-wasms/out/tree-sitter-html.wasm?url'),
  java: () => import('tree-sitter-wasms/out/tree-sitter-java.wasm?url'),
  javascript: () => import('tree-sitter-wasms/out/tree-sitter-javascript.wasm?url'),
  json: () => import('tree-sitter-wasms/out/tree-sitter-json.wasm?url'),
  kotlin: () => import('tree-sitter-wasms/out/tree-sitter-kotlin.wasm?url'),
  lua: () => import('tree-sitter-wasms/out/tree-sitter-lua.wasm?url'),
  objc: () => import('tree-sitter-wasms/out/tree-sitter-objc.wasm?url'),
  ocaml: () => import('tree-sitter-wasms/out/tree-sitter-ocaml.wasm?url'),
  php: () => import('tree-sitter-wasms/out/tree-sitter-php.wasm?url'),
  python: () => import('tree-sitter-wasms/out/tree-sitter-python.wasm?url'),
  rescript: () => import('tree-sitter-wasms/out/tree-sitter-rescript.wasm?url'),
  ruby: () => import('tree-sitter-wasms/out/tree-sitter-ruby.wasm?url'),
  rust: () => import('tree-sitter-wasms/out/tree-sitter-rust.wasm?url'),
  scala: () => import('tree-sitter-wasms/out/tree-sitter-scala.wasm?url'),
  solidity: () => import('tree-sitter-wasms/out/tree-sitter-solidity.wasm?url'),
  swift: () => import('tree-sitter-wasms/out/tree-sitter-swift.wasm?url'),
  systemrdl: () => import('tree-sitter-wasms/out/tree-sitter-systemrdl.wasm?url'),
  tlaplus: () => import('tree-sitter-wasms/out/tree-sitter-tlaplus.wasm?url'),
  toml: () => import('tree-sitter-wasms/out/tree-sitter-toml.wasm?url'),
  tsx: () => import('tree-sitter-wasms/out/tree-sitter-tsx.wasm?url'),
  typescript: () => import('tree-sitter-wasms/out/tree-sitter-typescript.wasm?url'),
  vue: () => import('tree-sitter-wasms/out/tree-sitter-vue.wasm?url'),
  yaml: () => import('tree-sitter-wasms/out/tree-sitter-yaml.wasm?url'),
  zig: () => import('tree-sitter-wasms/out/tree-sitter-zig.wasm?url'),
} satisfies Record<GrammarLanguage, () => Promise<{ default: string }>>
const queries = import.meta.glob<string>('./queries/*.scm', { query: '?raw', import: 'default' })

export const highlightFile = createTreeSitterHighlighter({
  runtimeWasm,
  async loadGrammar(language) {
    const { default: url } = await grammars[language]()
    const response = await fetch(url)
    if (!response.ok) throw new Error(`Unable to load ${language} grammar.`)
    return new Uint8Array(await response.arrayBuffer())
  },
  loadQuery: language => queries[`./queries/${language}.scm`](),
})
