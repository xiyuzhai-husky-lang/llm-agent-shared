import { App } from './app'
import './style.css'
const root = document.getElementById('app')
if (!root) throw new Error('Missing app root')
new App(root)
