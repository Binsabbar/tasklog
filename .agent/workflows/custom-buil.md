---
description: Use custom build command to mimick goreleaser build 

---

if I ask you to build using goreleaser way, use the command and ask me for the version then set it:
`go build -ldflags="-X main.version=1.0.0-alpha.6 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.builtBy=goreleaser" -o dist/tasklog ./main.go`
