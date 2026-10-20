import { expect, test } from 'bun:test'
import { createKathyVfs } from './index.ts'

test('shares pending and completed reads by path and oid', async () => {
  const calls: string[][] = []
  const vfs = createKathyVfs(async (path, oid) => {
    calls.push([path, oid])
    return `${path}@${oid}`
  })
  const first = vfs.read('a.rs', 'abcdef0')
  expect(vfs.read('a.rs', 'abcdef0')).toBe(first)
  expect(await first).toBe('a.rs@abcdef0')
  expect(vfs.read('a.rs', 'abcdef0')).toBe(first)
  expect(await vfs.read('a.rs', '1234567')).toBe('a.rs@1234567')
  expect(await vfs.read('b.rs', 'abcdef0')).toBe('b.rs@abcdef0')
  expect(calls).toEqual([['a.rs', 'abcdef0'], ['a.rs', '1234567'], ['b.rs', 'abcdef0']])
})

test('shares a failed read without swallowing the error or fetching again', async () => {
  let calls = 0
  const failure = new Error('missing source')
  const vfs = createKathyVfs(() => { calls++; throw failure })
  const first = vfs.read('missing.rs', 'abcdef0')
  await expect(first).rejects.toBe(failure)
  expect(vfs.read('missing.rs', 'abcdef0')).toBe(first)
  expect(calls).toBe(1)
})
