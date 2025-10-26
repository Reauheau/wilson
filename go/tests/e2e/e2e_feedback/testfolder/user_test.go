package main

import "testing"

func TestNewUser(t *testing.T) {
	user := NewUser(1, "testuser", "test@example.com")
	if user.ID != 1 {
		t.Errorf("Expected ID 1, got %d", user.ID)
	}
	if user.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", user.Username)
	}
}

func TestUser_Validate(t *testing.T) {
	tests := []struct {
		name    string
		user    *User
		wantErr bool
	}{
		{"valid user", &User{ID: 1, Username: "test", Email: "test@example.com"}, false},
		{"empty username", &User{ID: 1, Username: "", Email: "test@example.com"}, true},
		{"empty email", &User{ID: 1, Username: "test", Email: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.user.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("User.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
