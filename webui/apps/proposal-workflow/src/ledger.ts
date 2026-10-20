import type { Batch } from './batch'
import type { Member } from './member'
export interface Ledger { batches: Record<string, Batch>; members: Record<string, Member>; stack: string[] }
