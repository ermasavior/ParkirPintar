package handler

import (
	pb "parkir-pintar/services/presence/gen/presence/v1"
	"parkir-pintar/services/presence/internal/presence/usecase"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// PresenceServer implements the gRPC PresenceServiceServer interface
type PresenceServer struct {
	pb.UnimplementedPresenceServiceServer
	uc usecase.Presence
}

// NewPresenceServer creates a new PresenceServer
func NewPresenceServer(uc usecase.Presence) *PresenceServer {
	return &PresenceServer{uc: uc}
}

// validateUUID returns true if s is a valid UUID
func validateUUID(s string) bool {
	return validate.Var(s, "required,uuid") == nil
}
