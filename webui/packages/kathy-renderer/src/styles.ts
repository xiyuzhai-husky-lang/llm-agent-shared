// Everything a rendered Kathy document needs to look right: the renderer's own rules, the source frame's, and the
// math backend's (math/styles.ts, which carries its fonts). A page that ships server-rendered DOM imports this alone.
import './render.css'
import '@kathy/source/styles.css'
import './math/styles.ts'
