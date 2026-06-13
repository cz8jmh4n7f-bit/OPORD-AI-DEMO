package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
)

// CreateRequestInput is a self-service request for a resource/environment that
// must be approved before it is provisioned.
type CreateRequestInput struct {
	Name        string
	Environment string
	Requester   string
	Kind        string // vm | cluster | database | stack | environment
	Provider    string
	Blueprint   string          // for kind=environment
	Spec        json.RawMessage // kind-specific spec (VMSpec / ClusterSpec / ...)
}

var validRequestKinds = map[string]bool{
	"vm": true, "cluster": true, "database": true, "stack": true, "environment": true, "project": true, "account": true,
	"ai_service": true,
}

// CreateRequest records a pending request and (if a ticketer is wired) opens a
// GLPI ticket for it. Provisioning happens only after ApproveRequest.
func (s *Service) CreateRequest(ctx context.Context, in CreateRequestInput) (*db.Request, error) {
	if in.Name == "" || in.Provider == "" || in.Kind == "" {
		return nil, fmt.Errorf("request name, provider and kind are required")
	}
	if !validRequestKinds[in.Kind] {
		return nil, fmt.Errorf("invalid request kind %q (want vm|cluster|database|stack|environment|project|account|ai_service)", in.Kind)
	}
	env := in.Environment
	if env == "" {
		env = "dev"
	}
	spec := in.Spec
	if len(spec) == 0 {
		spec = json.RawMessage("{}")
	}

	req, err := s.q.CreateRequest(ctx, db.CreateRequestParams{
		Name:        in.Name,
		Environment: env,
		Requester:   in.Requester,
		Kind:        in.Kind,
		Provider:    in.Provider,
		Blueprint:   in.Blueprint,
		Spec:        spec,
		TenantID:    tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	s.log.Info("request created", "name", req.Name, "kind", req.Kind, "requester", req.Requester)

	// Open an ITSM ticket (best-effort).
	if s.ticketer != nil {
		title := fmt.Sprintf("OPORD request: %s %q (%s)", req.Kind, req.Name, env)
		content := fmt.Sprintf("Requester: %s\nKind: %s\nProvider: %s\nBlueprint: %s\nEnvironment: %s",
			req.Requester, req.Kind, req.Provider, req.Blueprint, env)
		if ticket, terr := s.ticketer.CreateTicket(ctx, title, content); terr != nil {
			s.log.Warn("ticket creation failed", "request", req.Name, "err", terr)
		} else if ticket != "" {
			if updated, uerr := s.q.SetRequestTicket(ctx, db.SetRequestTicketParams{ID: req.ID, TicketRef: ticket}); uerr == nil {
				req = updated
			}
		}
	}

	s.emit("request", "created", req.Name, env, req.Provider, fmt.Sprintf("%s requested by %s (ticket %s)", req.Kind, req.Requester, req.TicketRef))
	return &req, nil
}

// ApproveRequest approves a pending request and dispatches provisioning of the
// requested resource (which then flows to CMDB + Slack via the connector bus).
func (s *Service) ApproveRequest(ctx context.Context, name, env, approvedBy string) error {
	if env == "" {
		env = "dev"
	}
	req, err := s.q.GetRequestByName(ctx, db.GetRequestByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("request %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !tenantVisible(req.TenantID, tid) {
		return fmt.Errorf("request %q (env %q) not found", name, env)
	}
	if req.Status != "pending_approval" {
		return fmt.Errorf("request %q is %q, not pending approval", name, req.Status)
	}

	if _, err := s.q.DecideRequest(ctx, db.DecideRequestParams{ID: req.ID, Status: "approved", DecidedBy: approvedBy}); err != nil {
		return fmt.Errorf("recording approval: %w", err)
	}
	s.log.Info("request approved", "name", req.Name, "by", approvedBy)

	resourceRef, nextStatus, err := s.provisionRequest(ctx, req)
	if err != nil {
		_, _ = s.q.SetRequestResource(ctx, db.SetRequestResourceParams{ID: req.ID, ResourceRef: req.Name, Status: "failed"})
		s.emit("request", "failed", req.Name, env, req.Provider, err.Error())
		return fmt.Errorf("provisioning approved request: %w", err)
	}
	if resourceRef == "" {
		resourceRef = req.Name
	}
	if nextStatus == "" {
		nextStatus = "provisioning"
	}
	_, _ = s.q.SetRequestResource(ctx, db.SetRequestResourceParams{ID: req.ID, ResourceRef: resourceRef, Status: nextStatus})
	s.emit("request", "approved", req.Name, env, req.Provider, fmt.Sprintf("%s approved by %s", req.Kind, approvedBy))
	return nil
}

// provisionRequest dispatches to the right Create* by the request's kind. Each
// Create* provisions in the background.
func (s *Service) provisionRequest(ctx context.Context, req db.Request) (string, string, error) {
	switch req.Kind {
	case "vm":
		var spec models.VMSpec
		if err := json.Unmarshal(req.Spec, &spec); err != nil {
			return "", "", fmt.Errorf("decoding vm spec: %w", err)
		}
		_, err := s.CreateVM(ctx, CreateVMInput{Name: req.Name, Environment: req.Environment, Provider: req.Provider, Spec: spec})
		return req.Name, "provisioning", err
	case "cluster":
		var spec models.ClusterSpec
		if err := json.Unmarshal(req.Spec, &spec); err != nil {
			return "", "", fmt.Errorf("decoding cluster spec: %w", err)
		}
		_, err := s.CreateCluster(ctx, CreateClusterInput{Name: req.Name, Environment: req.Environment, Provider: req.Provider, Spec: spec})
		return req.Name, "provisioning", err
	case "database":
		var spec models.DatabaseSpec
		if err := json.Unmarshal(req.Spec, &spec); err != nil {
			return "", "", fmt.Errorf("decoding database spec: %w", err)
		}
		_, err := s.CreateDatabase(ctx, CreateDatabaseInput{Name: req.Name, Environment: req.Environment, Provider: req.Provider, Spec: spec})
		return req.Name, "provisioning", err
	case "stack":
		var spec models.StackSpec
		if err := json.Unmarshal(req.Spec, &spec); err != nil {
			return "", "", fmt.Errorf("decoding stack spec: %w", err)
		}
		_, err := s.CreateStack(ctx, CreateStackInput{Name: req.Name, Environment: req.Environment, Provider: req.Provider, Spec: spec})
		return req.Name, "provisioning", err
	case "project":
		var spec models.ProjectSpec
		if err := json.Unmarshal(req.Spec, &spec); err != nil {
			return "", "", fmt.Errorf("decoding project spec: %w", err)
		}
		_, err := s.CreateProject(ctx, CreateProjectInput{Name: req.Name, Environment: req.Environment, Provider: req.Provider, Spec: spec})
		return req.Name, "provisioning", err
	case "account":
		var spec models.AccountSpec
		if err := json.Unmarshal(req.Spec, &spec); err != nil {
			return "", "", fmt.Errorf("decoding account spec: %w", err)
		}
		_, err := s.CreateAccount(ctx, CreateAccountInput{Name: req.Name, Environment: req.Environment, Provider: req.Provider, Spec: spec})
		return req.Name, "provisioning", err
	case "ai_service":
		inst, err := s.ProvisionAIRequest(ctx, req)
		if err != nil {
			return "", "", err
		}
		return inst.ID.String(), "completed", nil
	case "environment":
		_, err := s.CreateEnvironment(ctx, CreateEnvironmentInput{Name: req.Name, Environment: req.Environment, Provider: req.Provider, Blueprint: req.Blueprint})
		return req.Name, "provisioning", err
	default:
		return "", "", fmt.Errorf("unknown request kind %q", req.Kind)
	}
}

// RejectRequest declines a pending request.
func (s *Service) RejectRequest(ctx context.Context, name, env, rejectedBy, reason string) error {
	if env == "" {
		env = "dev"
	}
	req, err := s.q.GetRequestByName(ctx, db.GetRequestByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("request %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !tenantVisible(req.TenantID, tid) {
		return fmt.Errorf("request %q (env %q) not found", name, env)
	}
	if req.Status != "pending_approval" {
		return fmt.Errorf("request %q is %q, not pending approval", name, req.Status)
	}
	if _, err := s.q.DecideRequest(ctx, db.DecideRequestParams{ID: req.ID, Status: "rejected", DecidedBy: rejectedBy, Reason: reason}); err != nil {
		return fmt.Errorf("recording rejection: %w", err)
	}
	s.log.Info("request rejected", "name", req.Name, "by", rejectedBy, "reason", reason)
	s.emit("request", "rejected", req.Name, env, req.Provider, strings.TrimSpace("rejected by "+rejectedBy+": "+reason))
	return nil
}

// ListRequests returns all requests, newest first.
func (s *Service) ListRequests(ctx context.Context) ([]db.Request, error) {
	rs, err := s.q.ListRequests(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing requests: %w", err)
	}
	tid, scoped := scopeTenant(ctx)
	if !scoped {
		return rs, nil
	}
	out := make([]db.Request, 0, len(rs))
	for _, r := range rs {
		if tenantVisible(r.TenantID, tid) {
			out = append(out, r)
		}
	}
	return out, nil
}

// RequestStatus returns one request by name + environment.
func (s *Service) RequestStatus(ctx context.Context, name, env string) (*db.Request, error) {
	if env == "" {
		env = "dev"
	}
	req, err := s.q.GetRequestByName(ctx, db.GetRequestByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("request %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !tenantVisible(req.TenantID, tid) {
		return nil, fmt.Errorf("request %q (env %q) not found", name, env)
	}
	return &req, nil
}
