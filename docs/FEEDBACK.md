1. GitHub CI has been failing. Need to figure this out
2. The golang json v2 might be worth adding her
3. go fix might be worth using for
  - https://go.dev/blog/gofix
  - https://go.dev/blog/inliner
4. should the poll interval be configurable 
5. I'm still not sure about how the harness would use the cli tool
  - for example: 
    - 1. the way you trigger a harness with a prompt. the way you make sure a harness uses the cli tools. a lot of the harnesses I use have hooks so that might be the way to do it
    - 2. also prviding some type of prime command like (beans prime) or skill would be helpful
    - maybe I can have an extras folder for those things that are harness specific like for opencode plugin, pi plugin, claude plugin, or maybe a hook(?) I think this is worth investigating.
    - because for some we are overwriting there built in features with this.
    - I think pi would be the easiest to do this, but some might be a bit more tricky
6. I have a question on how we handle an agent breaking or timing out.
---

i like it tho, it means later if there is something I wan tto change I can.
