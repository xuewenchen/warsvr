package handler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	userpb "cardwar/protocol/pb/user"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UserServiceServer implements pb.UserServiceServer with an in-memory store.
type UserServiceServer struct {
	userpb.UnimplementedUserServiceServer
	id     string
	mu     sync.RWMutex
	users  map[string]*userpb.User
	nextID atomic.Int64
}

// New creates a new UserServiceServer.
func New(instanceID string) *UserServiceServer {
	return &UserServiceServer{
		id:    instanceID,
		users: make(map[string]*userpb.User),
	}
}

func (s *UserServiceServer) Ping(ctx context.Context, req *userpb.PingRequest) (*userpb.PingResponse, error) {
	return &userpb.PingResponse{
		Message:  "pong",
		ServerId: fmt.Sprintf("user-service/%s", s.id),
	}, nil
}

func (s *UserServiceServer) CreateUser(ctx context.Context, req *userpb.CreateUserRequest) (*userpb.CreateUserResponse, error) {
	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	id := fmt.Sprintf("%d", s.nextID.Add(1))
	now := time.Now().Unix()

	u := &userpb.User{
		Id:        id,
		Username:  req.Username,
		Email:     req.Email,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.mu.Lock()
	s.users[id] = u
	s.mu.Unlock()

	return &userpb.CreateUserResponse{User: u}, nil
}

func (s *UserServiceServer) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
	s.mu.RLock()
	u, ok := s.users[req.Id]
	s.mu.RUnlock()
	if !ok {
		return nil, status.Errorf(codes.NotFound, "user %s not found", req.Id)
	}
	return &userpb.GetUserResponse{User: u}, nil
}

func (s *UserServiceServer) UpdateUser(ctx context.Context, req *userpb.UpdateUserRequest) (*userpb.UpdateUserResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.users[req.Id]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "user %s not found", req.Id)
	}

	if req.Username != "" {
		u.Username = req.Username
	}
	if req.Email != "" {
		u.Email = req.Email
	}
	u.UpdatedAt = time.Now().Unix()

	return &userpb.UpdateUserResponse{User: u}, nil
}

func (s *UserServiceServer) DeleteUser(ctx context.Context, req *userpb.DeleteUserRequest) (*userpb.DeleteUserResponse, error) {
	s.mu.Lock()
	delete(s.users, req.Id)
	s.mu.Unlock()
	return &userpb.DeleteUserResponse{Success: true}, nil
}

func (s *UserServiceServer) ListUsers(ctx context.Context, req *userpb.ListUsersRequest) (*userpb.ListUsersResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pageSize := int(req.PageSize)
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	keys := make([]string, 0, len(s.users))
	for k := range s.users {
		keys = append(keys, k)
	}

	start := 0
	if req.PageToken != "" {
		start = len(req.PageToken) // simplified: offset-based
	}

	var users []*userpb.User
	var nextToken string
	for i := start; i < len(keys) && len(users) < pageSize; i++ {
		users = append(users, s.users[keys[i]])
	}
	if start+len(users) < len(keys) {
		nextToken = fmt.Sprintf("%d", start+len(users))
	}

	return &userpb.ListUsersResponse{
		Users:         users,
		NextPageToken: nextToken,
	}, nil
}
