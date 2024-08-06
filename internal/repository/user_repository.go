package repository

import (
	"fmt"
	"github.com/asb1302/innopolis_go_assesment_1/internal/types"
)

type UserRepository struct {
	users       map[string]types.User
	validTokens map[string]bool
}

func NewUserRepository(validTokens []string) *UserRepository {
	tokenMap := make(map[string]bool)
	usersMap := make(map[string]types.User)
	for _, token := range validTokens {
		tokenMap[token] = true
	}
	return &UserRepository{
		validTokens: tokenMap,
		users:       usersMap,
	}
}

func (r *UserRepository) IsValidToken(token string) bool {
	return r.validTokens[token]
}

func (r *UserRepository) AddUser(user types.User) error {
	if _, exists := r.users[user.Token]; exists {
		return fmt.Errorf("токен уже используется другим пользователем")
	}
	r.users[user.Token] = user
	r.validTokens[user.Token] = true

	return nil
}

func (r *UserRepository) GetUserByToken(token string) (types.User, bool) {
	user, exists := r.users[token]
	return user, exists
}
