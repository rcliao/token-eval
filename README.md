# token-eval

Capture and query LLM call records for prompt evaluation. Part of the [teeny-claw](https://github.com/rcliao/teeny-claw) constellation.

## Install

```bash
go install github.com/rcliao/token-eval/cmd/token-eval@latest
```

Or build from source:

```bash
make build
# Binary at ./bin/token-eval
```

## Quick Start

```bash
# Record an LLM call
token-eval record -p my-project -m claude-sonnet-4 -i 1000 -o 500 \
  --prompt "Implement search" --intent "Add FTS5 search" --result pass

# Record with stdin JSON (for full prompt/context/output)
cat <<EOF | token-eval record -p my-project -m claude-sonnet-4
{
  "task": "implement-search",
  "prompt": "Add FTS5 search to the store layer",
  "context": "Project uses SQLite...",
  "intent": "Search should match and return results",
  "output": "Here's the implementation...",
  "input_tokens": 2500,
  "output_tokens": 800,
  "result": "pass",
  "quality": 90
}
EOF

# Query records
token-eval query -p my-project
token-eval query -p my-project --result pass --full
token-eval query "FTS5 search"    # full-text search

# Manage pricing
token-eval price list
token-eval price set custom-model --input 5.0 --output 20.0
```

## Commands

| Command | Description |
|---------|-------------|
| `record` | Capture an LLM call with prompt, context, intent, output |
| `query` | Search and filter captured records |
| `price list` | Show all model pricing |
| `price set` | Add or update model pricing |
| `price rm` | Remove a model's pricing |

## Database

Default location: `~/.token-eval/eval.db`

Override with `--db` flag or `TOKEN_EVAL_DB` environment variable.

## Bundled Pricing

Ships with pricing for: Claude Opus/Sonnet/Haiku, GPT-4o/mini, Gemini 2.5 Pro, DeepSeek R1. Costs are auto-computed on record insertion.

## Dependencies

- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — Pure Go SQLite
- [github.com/oklog/ulid/v2](https://github.com/oklog/ulid) — ULID generation
- [github.com/spf13/cobra](https://github.com/spf13/cobra) — CLI framework

## License

MIT
