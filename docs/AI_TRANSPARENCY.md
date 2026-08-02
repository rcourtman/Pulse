# AI-Assisted Development

Pulse is built with AI tools, extensively and deliberately. This page exists so
there is no ambiguity about that, and so nobody has to guess.

## The short version

I am the sole maintainer of Pulse. I use AI assistance throughout the project.
Code, tests, documentation, release notes, and issue replies may all be
produced with AI tools in the loop. Everything ships under my direction, my
review, and my responsibility. If something is wrong, it is my bug, regardless
of what typed the first draft.

## Why

Pulse monitors Proxmox, PBS, Docker, Kubernetes, TrueNAS, and vSphere. Around
that sits a web UI, a mobile app, host agents, installers, release tooling,
security processes, and a support queue. That surface area is well beyond what
one person can sustain by hand at any reasonable quality bar.

AI assistance is the reason a solo project can cover that breadth, ship
frequent releases, and usually turn a reproducible bug report into a fix in
days rather than months. Refusing these tools would not make Pulse better. It
would make it slower and buggier. I consider working this way a professional
obligation, not a shortcut.

## Where it is used

Declaring the general position is easy. Declaring where it applies is more
useful, so here is the breakdown.

- **Code and tests.** Much of both is written with AI coding tools, working
  from my direction and landing through the same review, test, and audit
  gates as anything else.
- **Documentation and release notes.** Drafted with AI assistance and checked
  against the actual code and behaviour before publishing.
- **Issue triage and support.** AI helps investigate reports, reproduce bugs,
  and draft replies. When a response comes from an automated triage run
  rather than from me at the keyboard, it says so in the reply itself. I
  would rather you know you are reading automation than wonder whether you
  are.

## What stays human

The tools write a lot of the code. They do not decide what Pulse is. Product
direction, architecture, what gets built and what gets cut, review of what
ships, and every release decision are mine. When you report a bug, the
investigation may be automated, but what the fix should be and whether it is
good enough to ship is my call, and I own the outcome of every thread either
way.

## What it does not change

Pulse is judged the same way all software should be judged, by what it does.
The test suites, the audit gates that run before anything lands, the release
candidate process, and the issue tracker history are all public. If you find a
bug, report it with steps to reproduce and it will get fixed. "This looks
AI-written" is not a bug report, and style-sniffing a diff or an issue reply
tells you nothing about whether the code is correct.

## From here on

This document sets the standard from the day it landed, not a claim about the
past. I used AI for a long time before writing any of this down, and some of
that use was not declared, including automated issue replies that carried
nothing to say they were automated. I am not going to rewrite history or
pretend otherwise. What I can do is state the rules now and hold to them going
forward.

## If you object on principle

That is your call, and I am not going to argue anyone out of it. But I am also
not going to pretend, hedge, or apologise. Nearly every actively developed
software project you rely on today is built with some level of AI assistance,
whether its maintainers say so or not. I would rather say so.

Judge Pulse on what it does on your infrastructure and on the track record in
the [issue tracker](https://github.com/rcourtman/Pulse/issues). Those are the
things that matter, and they are the things I stand behind.

For what Pulse itself does with AI as a product (Pulse Patrol, Assistant, and
MCP, all optional and off by default until you configure a provider), see
[AI.md](AI.md).
