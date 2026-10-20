export interface KathyVfs {
  read(path: string, oid: string): Promise<string>
}

/** Share one instance across the application's documents. Reads, including pending
 * reads and failures, are retained once per path and revision. */
export function createKathyVfs(load: KathyVfs['read']): KathyVfs {
  const sources = new Map<string, Promise<string>>()
  return {
    read(path, oid) {
      const key = JSON.stringify([path, oid])
      let source = sources.get(key)
      if (source === undefined) {
        source = Promise.resolve().then(() => load(path, oid))
        sources.set(key, source)
      }
      return source
    },
  }
}
