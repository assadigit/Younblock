// Package platform picks the Platform implementation for this build.
// Phase 3 replaces the stub with the real Windows adapter behind a
// windows-only build tag; until then every build uses the stub, which is
// why GOOS=windows builds stay green during Phase 1 (DECISIONS.md D12).
package platform

import (
	"github.com/assadigit/Younblock/internal/engine"
	"github.com/assadigit/Younblock/internal/platform/stub"
)

// New returns the platform adapter for this build.
func New() engine.Platform { return stub.New() }
