package main

import "testing"

func TestNewHandler(t *testing.T) {
	handler := NewHandler()
	if handler == nil {
		t.Error("Expected handler to be created")
	}
	if handler.users == nil {
		t.Error("Expected users map to be initialized")
	}
}

func TestHandler_AddUser(t *testing.T) {
	handler := NewHandler()
	user := NewUser(1, "testuser", "test@example.com")

	err := handler.AddUser(user)
	if err != nil {
		t.Errorf("Failed to add user: %v", err)
	}
}

func TestHandler_GetUser(t *testing.T) {
	handler := NewHandler()
	user := NewUser(1, "testuser", "test@example.com")
	handler.AddUser(user)

	retrieved, err := handler.GetUser(1)
	if err != nil {
		t.Errorf("Failed to get user: %v", err)
	}
	if retrieved.ID != user.ID {
		t.Errorf("Expected ID %d, got %d", user.ID, retrieved.ID)
	}
}
