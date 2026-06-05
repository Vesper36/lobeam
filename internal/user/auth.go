package user

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/vesper/mimo/internal/db"
	"github.com/vesper/mimo/internal/model"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidToken       = errors.New("invalid token")
)

type Service struct {
	db        *db.DB
	jwtSecret []byte
	jwtExpiry time.Duration
	refreshExpiry time.Duration
}

func NewService(database *db.DB, jwtSecret string, jwtExpiry, refreshExpiry time.Duration) *Service {
	return &Service{
		db:            database,
		jwtSecret:     []byte(jwtSecret),
		jwtExpiry:     jwtExpiry,
		refreshExpiry: refreshExpiry,
	}
}

type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func (s *Service) Register(username, email, password, role string) (*model.User, error) {
	// Check if user exists
	if _, err := s.db.GetUserByUsername(username); err == nil {
		return nil, ErrUserExists
	}
	if _, err := s.db.GetUserByEmail(email); err == nil {
		return nil, ErrUserExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	if role == "" {
		role = "member"
	}

	u := &model.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
	}
	if err := s.db.CreateUser(u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) Login(username, password string) (accessToken, refreshToken string, err error) {
	u, err := s.db.GetUserByUsername(username)
	if err != nil {
		return "", "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", "", ErrInvalidCredentials
	}

	accessToken, err = s.generateToken(u, s.jwtExpiry)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = s.generateToken(u, s.refreshExpiry)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (s *Service) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (s *Service) RefreshToken(refreshToken string) (string, error) {
	claims, err := s.ValidateToken(refreshToken)
	if err != nil {
		return "", err
	}
	u, err := s.db.GetUserByID(claims.UserID)
	if err != nil {
		return "", ErrInvalidToken
	}
	return s.generateToken(u, s.jwtExpiry)
}

func (s *Service) GetUser(id int64) (*model.User, error) {
	return s.db.GetUserByID(id)
}

func (s *Service) generateToken(u *model.User, expiry time.Duration) (string, error) {
	claims := &Claims{
		UserID:   u.ID,
		Username: u.Username,
		Role:     u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}
