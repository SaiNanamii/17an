package service

import (
	"17an/models"
	"17an/repository"
)

type UserService interface {
	ListUsers(page, limit int) ([]models.User, error)
	GetUser(id uint64) (*models.User, error)
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) ListUsers(page, limit int) ([]models.User, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.FindAll(limit, (page-1)*limit)
}

func (s *userService) GetUser(id uint64) (*models.User, error) {
	return s.repo.FindByID(id)
}
