package storage

import "psv-crowd-counter/internal/core/models"

type Store interface {
	Save(models.Report) error
	List() ([]models.Report, error)
}
