package main

import "fmt"

type User struct {
	ID       int
	Username string
	Email    string
}

func NewUser(id int, username, email string) *User {
	return &User{
		ID:       id,
		Username: username,
		Email:    email,
	}
}

func (u *User) Validate() error {
	if u.Username == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if u.Email == "" {
		return fmt.Errorf("email cannot be empty")
	}
	return nil
}
