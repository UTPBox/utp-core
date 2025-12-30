package psiphon

import (
	"context"
	"encoding/json"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	
	// Try loading libbox to see if it triggers registration or offers API
	// _ "github.com/sagernet/sing-box/experimental/libbox" 
)

func init() {
	// Trying option.RegisterOutbound. If this fails, we need to find the specific registry.
	// option.RegisterOutbound("psiphon", NewOutboundWrapper)
	// For now, allow build to pass.
}

// Exported constructor
func NewOutboundWrapper(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, rawMessage json.RawMessage) (adapter.Outbound, error) {
	return NewOutbound(ctx, router, logger, tag, rawMessage)
}
