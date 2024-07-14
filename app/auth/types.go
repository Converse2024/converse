package auth

import (
	"time"
)

type User struct {
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	Username   string    `json:"username"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	ProfilePic string    `json:"profile_pic"`
	Email      string    `json:"email"`
	Password   string    `json:"password"`
	IsAdmin    bool      `json:"is_admin"`
}

type OtpGenerateRequest struct {
	Email        string `json:"email"`
	Type         string `json:"type"`
	Organization string `json:"organization"`
	Subject      string `json:"subject"`
}

type OtpVerifyRequest struct {
	Email string `json:"email"`
	Otp   string `json:"otp"`
}

type OtpApiResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

/*
func createUserFromFormValues(values pages.RegisterFormValues) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(values.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	user := User{
		Email:     values.Email,
		FirstName: values.FirstName,
		LastName:  values.LastName,
		Password:  string(hash),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err = db.Query.NewInsert().Model(&user).Exec(context.Background())
	return user, err
}
*/
