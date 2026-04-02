This directory defines the high-level concepts, business logic, and architecture of this project using markdown. It is managed by [lat.md](https://www.npmjs.com/package/lat.md) — a tool that anchors source code to these definitions. Install the `lat` command with `npm i -g lat.md` and run `lat --help`.

- [[architecture]] — unified CLI design, module structure, and core design principles
- [[modules]] — deep module descriptions with key interfaces and storage decisions
- [[state]] — `.swarm/` directory layout and storage patterns
- [[backends]] — multiplexer backend interface, detection cascade, and per-backend coverage
- [[extras]] — agent skill, Claude Code hooks, opencode plugin, and pi extension
- [[site]] — GitHub Pages landing page: visual design, content structure, interactions, and technical choices
