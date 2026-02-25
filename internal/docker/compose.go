package docker

import (
	"os"
)

// ComposeTemplate is the standard template for the Qdrant MCP server
const ComposeTemplate = `services:
  qdrant-mcp:
    image: ${IMAGE:-qdrant-mcp-server-mcp-server}
    user: root
    command: /bin/sh -c "apk add --no-cache git && node build/index.js"
    extra_hosts:
      - "host.docker.internal:host-gateway"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - QDRANT_API_KEY=${QDRANT_API_KEY}
      - EMBEDDING_MODEL=${EMBEDDING_MODEL}
      - TRANSPORT_MODE=${TRANSPORT_MODE}
      - EMBEDDING_PROVIDER=${EMBEDDING_PROVIDER}
      - EMBEDDING_BASE_URL=${OPENAI_BASE_URL}
      - OPENAI_BASE_URL=${OPENAI_BASE_URL}
      - LOG_LEVEL=${LOG_LEVEL}
      - QDRANT_URL=${QDRANT_URL}
      - HTTP_PORT=${HTTP_PORT}
      - EMBEDDING_DIMENSIONS=${EMBEDDING_DIMENSIONS}
    volumes:
      - ${PROJECT_PATH}:/app/project:ro
    ports:
      - "${HTTP_PORT}:3000"
`

// GenerateComposeFile writes the standard template to the specified path
func GenerateComposeFile(path string) error {
	// Only write if it doesn't exist to prevent overriding manual user tweaks unless forced
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(ComposeTemplate), 0644)
}
