---
name: claude-only-agent
description: An agent that only targets Claude Code
model: opus
tools:
  - Read
  - Write
  - Bash
color: purple
useonly: claude

claude:
  permissionMode: bypassPermissions
  maxTurns: 50
  effort: max
---

## Claude-Only Agent

This agent is only deployed to Claude Code.
