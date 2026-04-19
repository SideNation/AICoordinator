---
name: edge-namespace-override
description: Agent where namespace blocks override shared fields
model: sonnet
tools:
  - Read
color: blue

claude:
  model: opus
  tools: Read, Write, Bash
  color: red

opencode:
  model: anthropic/claude-haiku-4-5-20251001
  tools:
    - Grep
  color: "#ef4444"
---

Namespace override test.
