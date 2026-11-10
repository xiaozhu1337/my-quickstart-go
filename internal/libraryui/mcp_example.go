package libraryui

// Keep optional example fields commented so the default remains ready to save.
const mcpExample = `# command: executable name or absolute path; args: one argument per list item.
command: pnpm
args:
  - dlx
  - '@modelcontextprotocol/server-filesystem'
  - 'C:/workspace'

` + mcpHelp

const mcpHelp = `# Optional stdio fields (remove the leading # to enable):
# env: environment variables; indent keys by two spaces and quote values.
# env:
#   API_KEY: 'replace-with-your-key'
#   LOG_LEVEL: 'info'
#   PORT: '3000'
# cwd: absolute working directory; use forward slashes on Windows.
# cwd: 'C:/workspace/my-project'
# Values are literal strings; this app does not expand ${VAR} placeholders.
# Environment variables and headers are saved as plain text in the library.
#
# HTTP example: remove command/args/env/cwd, then uncomment url/headers.
# url: 'https://example.com/mcp'
# headers:
#   Authorization: 'Bearer replace-with-your-token'
#   X-API-Key: 'replace-with-your-key'
# Choose command (stdio) OR url (HTTP), never both.
`
