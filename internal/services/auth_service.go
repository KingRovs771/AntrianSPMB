package service

import (
	"AntrianSPMB/internal/models"
	"AntrianSPMB/internal/repository"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Login(username, password string) (*models.User, error)
	GenerateToken(user *models.User) (string, error)
	HashPassword(password string) (string, error)
}

type authService struct {
	userRepo repository.UserRepository
}

func NewAuthService(ur repository.UserRepository) AuthService {
	return &authService{userRepo: ur}
}

func (s *authService) Login(username, password string) (*models.User, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, errors.New("username atau password salah")
	}

	// Bandingkan password
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, errors.New("username atau password salah")
	}

	return user, nil
}

func (s *authService) GenerateToken(user *models.User) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "default_secret_key"
	}

	claims := jwt.MapClaims{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func (s *authService) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}
