# HTML Email Rendering

While Postero is a terminal-first email client, many modern emails are sent in HTML format. We allow you to delegate HTML rendering to external text-based web browsers.

## Recommended Tools

You can use any of the following tools to render HTML emails cleanly in the terminal:
- **w3m** (`brew install w3m` or `apt install w3m`)
- **lynx** (`brew install lynx` or `apt install lynx`)
- **elinks** (`brew install elinks` or `apt install elinks`)

## Setup

Add MIME filters to your `config.yaml`:

```yaml
filters:
	text/html: "w3m -T text/html -dump"
	# Optional plain-text cleanup step
	# text/plain: "sed -e 's/\\r$//'"
```

Postero will prefer the `text/html` filter when the message contains HTML, and fall back to plain text otherwise.

If the external command fails, Postero falls back to the raw message body instead of aborting the view.
