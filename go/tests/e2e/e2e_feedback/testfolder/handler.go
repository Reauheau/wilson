package main

import "fmt"

type Handler struct {
	users map[int]*User
}

func NewHandler() *Handler {
	return &Handler{
		users: make(map[int]*User),
	}
}

func (h *Handler) AddUser(user *User) error {
	if err := user.Validate(); err != nil {
		return fmt.Errorf("invalid user: %w", err)
	}
	h.users[user.ID] = user
	return nil
}

func (h *Handler) GetUser(id int) (*User, error) {
	user, exists := h.users[id]
	if !exists {
		return nil, fmt.Errorf("user not found: %d", id)
	}
	return user, nil
}
