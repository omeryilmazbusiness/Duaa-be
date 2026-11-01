package authn

// Bcrypt satisfies application password ports without leaking crypto into domain.
type Bcrypt struct{}

func (Bcrypt) Hash(plain string) (string, error) { return HashPassword(plain) }

func (Bcrypt) Check(hash, plain string) bool { return CheckPassword(hash, plain) }
