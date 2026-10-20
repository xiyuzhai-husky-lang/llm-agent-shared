import { useSyncExternalStore } from 'react'
const opened=new Set<string>()
const listeners=new Set<()=>void>()
export function markDiffOpened(candidate:string){if(opened.has(candidate))return;opened.add(candidate);for(const listener of listeners)listener()}
export function diffWasOpened(candidate:string){return opened.has(candidate)}
export function useDiffWasOpened(candidate:string){return useSyncExternalStore(listener=>{listeners.add(listener);return()=>{listeners.delete(listener)}},()=>diffWasOpened(candidate))}
