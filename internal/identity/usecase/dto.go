package usecase

type RegisterInput struct {
	Email     string
	Password  string
	FirstName string // forwarded to user-sv via identity.registered event
	LastName  string // forwarded to user-sv via identity.registered event
}

type RegisterOutput struct {
	IdentityID   string
	Email        string
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
}
