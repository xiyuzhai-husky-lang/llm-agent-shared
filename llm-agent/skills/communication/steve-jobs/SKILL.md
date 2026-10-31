---
name: steve-jobs
description: How to say anything a human reads — a chat report, a web page, a slide, a pitch. Get to the point the way Steve Jobs did: the one thing that matters first, in plain words, from the reader's side. Load before writing any message, page or presentation for the user or for an audience.
---

# Say it like Steve Jobs

The reader's bandwidth is small. Every word you add costs attention that the point needed. Get to the point.

## The rules

1. **The point first.** The first line carries the most important thing. A reader who stops after one sentence has the message.
2. **One message.** Decide what the single most important thing is before writing. Everything else is cut, or put behind a link or a collapsed section.
3. **From the reader's side, not the mechanism.** Start from what the reader pays or gains, never from how it works inside. The mechanism is for whoever asks.
4. **No technical detail the reader did not ask for.** No spec section numbers, instruction names, file paths, theorem names, counts or process narration in a pitch, a page lead or a report.
5. **Do not split one thought into points.** A list is for things that are truly separate. One idea is one sentence.
6. **Headings are the points.** On a page, a reader who reads only the headings gets the whole argument.
7. **Short.** No paragraph longer than three lines on a page. A chat report is a few lines.

## The model

The user's own pitch for formal verification of GPU programs (2026-10-31):

> Testing has to be redone for every new GPU, every small difference between them, and still cannot cover everything. A formal proof does it once, far more efficiently, and it shows exactly which properties of the hardware your program depends on.

Why it works: it starts from the reader's cost (testing every GPU, forever), names the value (once, for all of them), and adds one concrete payoff. No mechanism, no jargon.

What it replaced: "A kernel's correctness is a claim about every execution the PTX ISA allows on every chip that implements it", and before that a page titled "Beyond testing" with twelve dense bug write-ups. Both start from the mechanism; the reader has to learn the machinery before seeing why it matters. "Beyond testing" also framed the work as testing plus extras, which it is not.

## Page, slides, presentation

- **Web page:** the lead is the one message; the evidence sits below, collapsed or linked. The page opens short.
- **Slides:** one idea per slide; the title is the idea.
- **Presentation:** say the conclusion first, then why the listener should care, then stop.

## Check before sending

- Does the first line carry the point?
- Would a business leader, not an engineer, understand it and see why it matters?
- What can be deleted? Delete it.

User, 2026-10-31: "Go get the fucking point", "不要在这里扯一些没用的技术细节", "第一眼就得看到最重要的东西", "站在一个商业领袖的角度，像乔布斯一样说话".
