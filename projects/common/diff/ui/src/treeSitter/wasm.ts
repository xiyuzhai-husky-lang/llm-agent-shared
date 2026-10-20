// tree-sitter-wasms 0.1.13 includes legacy Emscripten `dylink` headers.
// web-tree-sitter 0.27 expects `dylink.0`. Translate only that custom section;
// the grammar's code, data and Tree-sitter ABI are unchanged.
export function compatibleGrammarWasm(bytes: Uint8Array): Uint8Array {
  bytes = compatibleCppImports(bytes)
  if (bytes[8] !== 0) return bytes
  let offset = 9
  const readLeb = () => {
    let value = 0
    let shift = 0
    let byte: number
    do {
      byte = bytes[offset++]
      value |= (byte & 127) << shift
      shift += 7
    } while (byte & 128)
    return value
  }
  const size = readLeb()
  const end = offset + size
  const nameLength = readLeb()
  const name = new TextDecoder().decode(bytes.subarray(offset, offset + nameLength))
  if (name !== 'dylink') return bytes
  offset += nameLength
  const start = offset
  for (let index = 0; index < 4; index++) readLeb()
  const memory = bytes.subarray(start, offset)
  const needed = bytes.subarray(offset, end)
  const section = new Uint8Array([
    8, ...new TextEncoder().encode('dylink.0'),
    1, ...leb(memory.length), ...memory,
    2, ...leb(needed.length), ...needed,
  ])
  const header = new Uint8Array([0, ...leb(section.length)])
  const result = new Uint8Array(8 + header.length + section.length + bytes.length - end)
  result.set(bytes.subarray(0, 8))
  result.set(header, 8)
  result.set(section, 8 + header.length)
  result.set(bytes.subarray(end), 8 + header.length + section.length)
  return result
}

// The older YAML scanner imports C++ new/delete. The current runtime exposes
// their C allocation equivalents. Its length-error handler terminates via abort.
const cppImports: Record<string, string> = {
  _Znwm: 'malloc',
  _ZdlPv: 'free',
  _ZNKSt3__220__vector_base_commonILb1EE20__throw_length_errorEv: 'abort',
}

function compatibleCppImports(bytes: Uint8Array): Uint8Array {
  let offset = 8
  const readLeb = () => {
    let value = 0
    let shift = 0
    let byte: number
    do {
      byte = bytes[offset++]
      value |= (byte & 127) << shift
      shift += 7
    } while (byte & 128)
    return value
  }
  const readName = () => {
    const length = readLeb()
    const name = new TextDecoder().decode(bytes.subarray(offset, offset + length))
    offset += length
    return name
  }
  const limits = () => { const flags = readLeb(); readLeb(); if (flags & 1) readLeb() }
  while (offset < bytes.length) {
    const sectionStart = offset
    const kind = bytes[offset++]
    const size = readLeb()
    const start = offset
    const end = start + size
    if (kind !== 2) { offset = end; continue }
    const count = readLeb()
    const replacements: { start: number; end: number; bytes: Uint8Array }[] = []
    for (let index = 0; index < count; index++) {
      const module = readName()
      const nameStart = offset
      const name = readName()
      if (module === 'env' && Object.hasOwn(cppImports, name)) {
        const nameBytes = new TextEncoder().encode(cppImports[name])
        replacements.push({ start: nameStart, end: offset, bytes: new Uint8Array([...leb(nameBytes.length), ...nameBytes]) })
      }
      switch (bytes[offset++]) {
        case 0: readLeb(); break // function type
        case 1: offset++; limits(); break // table
        case 2: limits(); break // memory
        case 3: offset += 2; break // global
        default: return bytes
      }
    }
    if (replacements.length === 0) return bytes
    const payload: number[] = []
    let cursor = start
    for (const replacement of replacements) {
      payload.push(...bytes.subarray(cursor, replacement.start), ...replacement.bytes)
      cursor = replacement.end
    }
    payload.push(...bytes.subarray(cursor, end))
    const header = new Uint8Array([2, ...leb(payload.length)])
    const result = new Uint8Array(sectionStart + header.length + payload.length + bytes.length - end)
    result.set(bytes.subarray(0, sectionStart))
    result.set(header, sectionStart)
    result.set(payload, sectionStart + header.length)
    result.set(bytes.subarray(end), sectionStart + header.length + payload.length)
    return result
  }
  return bytes
}

function leb(value: number): number[] {
  const bytes: number[] = []
  do {
    const byte = value & 127
    value >>>= 7
    bytes.push(byte | (value ? 128 : 0))
  } while (value)
  return bytes
}
