package model

type User struct {
	ID        int    `json:"id"`
	Email     string `json:"email"`
	Password  string `json:"-"`
	Role      string `json:"role"`
	StudentID *int   `json:"-"`
}

type Identity struct {
	UserID    int
	Role      string
	StudentID *int
}
