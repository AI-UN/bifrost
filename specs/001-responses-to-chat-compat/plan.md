# Implementation Plan: Responses To Chat Compatibility

**Branch**: `[compat/convert-responses-to-chat]` | **Date**: 2026-05-06 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/001-responses-to-chat-compat/spec.md`

## Summary

Add a new compatibility flag that lets Bifrost convert OpenAI Responses API requests into chat-completions requests when the selected upstream model supports chat but not Responses. Reuse the existing request/response conversion helpers in `core/schemas`, wire the new flag through config persistence, header overrides, compat plugin reloads, and the compatibility settings UI, and cover both non-streaming and streaming request paths with focused tests.

## Technical Context

**Language/Version**: Go 1.26.x, TypeScript/React in the existing Vite UI  
**Primary Dependencies**: Bifrost core schemas and routing, compat plugin, configstore, FastHTTP transport handlers, React settings UI  
**Storage**: Existing config store persistence for `client_config.compat`  
**Testing**: Go unit tests and targeted package tests; UI validation through type-level build coverage and config view logic review  
**Target Platform**: Linux server for backend, web browser for UI  
**Project Type**: Multi-module Go workspace with React admin UI  
**Performance Goals**: Preserve existing compatibility behavior without adding extra routing passes outside the conversion path  
**Constraints**: Must not break existing `convert_text_to_chat`, `convert_chat_to_responses`, per-provider Responses fallbacks, or streaming shape conversion  
**Scale/Scope**: Backend config, routing, plugin wiring, schema helpers, docs, and one UI settings page

## Constitution Check

The repository's `.specify/memory/constitution.md` is still an unfilled template, so there are no project-specific constitutional gates to enforce beyond the repository's existing coding and testing guidance.

## Project Structure

### Documentation (this feature)

```text
specs/001-responses-to-chat-compat/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── compatibility-config.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
core/
├── bifrost.go
├── utils.go
└── schemas/
    ├── bifrost.go
    └── mux.go

framework/
└── configstore/
    ├── clientconfig.go
    └── tables/clientconfig.go

plugins/
└── compat/
    └── main.go

transports/
├── config.schema.json
└── bifrost-http/
    ├── handlers/config.go
    └── lib/ctx.go

ui/
├── app/workspace/config/views/compatibilityView.tsx
└── lib/types/config.ts

docs/
└── features/compat-plugin.mdx
```

**Structure Decision**: This feature spans the existing Go backend modules plus the current React configuration UI and compatibility documentation. No new top-level modules are needed.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
