---
name: picture-first
description: How to make a web page that presents something (a site section, a case, an incident, a pitch) — it is a presentation, not a textbook; the illustration carries the story, with real logos and as much information as a picture can hold; words only for what a picture cannot say. Load with steve-jobs before designing or briefing any page, slide or design candidate the user will look at.
---

# Picture first

A page we show is a presentation built with HTML, not a textbook. The reader must know what it is at first glance, and the picture should tell most of the story.

## The rules

1. **First glance tells what it is.** With nothing clicked and nothing scrolled, a reader knows in about five seconds what the page is about and what it shows.
2. **The picture is the story.** Whatever a picture can say, do not say in words. Put as much information as possible into the illustration: who, what, on what hardware, the mechanism, the outcome, the time span. Outside the picture, keep a headline, at most one short line, and the source.
3. **Name the parties with their real logos.** If Cloudflare had the incident, the Cloudflare logo is in the picture. If SGLang had the bug, the SGLang logo is there. The same goes for the hardware vendor (NVIDIA). A reader should see whose system it is without reading a word. Use the official logo files, embedded as data URIs so the page makes no outside requests.
4. **Draw the real thing.** Draw a GPU when the point is a GPU, and draw its memory when the bug is in memory. Do not use abstract boxes labelled "service" or "shared memory", and do not ship a quick sketch when a real picture or icon exists.
5. **Write events like news.** The headline states what happened and why it matters. A reader who stops after one or two sentences of lede has the whole story. Do not use form labels ("What happened / Why tests missed it / What it cost") or field-by-field write-ups.
6. **Navigable, not a chronicle.** Split long material into chapters and into items the reader opens or jumps to. Never one long scroll of everything.
7. **Not fancy.** This is not an advertisement. Calm and clear, with no decoration that carries no information.
8. **Stay on the project's subject.** Examples and stories must be about what the project does. For gellar, that is GPU code. A story that drifts to general "AI infrastructure" without showing the GPU code at fault is off target.

## Making design candidates

- When a whole page keeps failing, narrow down: take one item, for example one incident, and make many versions of just that until the form is right. Then apply the form to the rest.
- Run several designer models (currently Fable, Astra, i.e. Codex, and Argon), five versions each, all genuinely different from one another.
- Give every version pass/fail buttons with Y/N shortcuts (demo tabs) so the user can rule quickly.
- Before sending, look at every candidate against the rules above. If none shows the logos, or none lets the picture carry the story, fix the brief and rerun. Do not send them.

## Check before sending

- Cover the text. Does the picture alone say who, what and what went wrong?
- Are the real logos of every named party in the picture?
- Can any sentence be replaced by something drawn? Then replace it.
- Is it about the project's own subject?

User, 2026-11-02: "本质上我们在用网页做 PPT、在做 presentation，不是让你写教材", "我第一眼就知道这大概是什么东西", "你知不知道怎么写新闻？", "你用 Cloudflare 出事故，那你 Cloudflare 图标在哪里？SGLang 的问题出事故，SGLang 的图标在哪里？", "illustration 里面尽可能多放信息，能用图说清楚的就不要用文字", "必须是 GPU coding 相关的". Earlier: "我不喜欢太 fancy 的，我们这不是在做广告", "我让你画 GPU，你就随便糊弄？你找不到图吗？", "不要所有事情都搞成一个流水账".
