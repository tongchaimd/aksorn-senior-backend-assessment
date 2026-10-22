package course

import (
	"context"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) RecordAttempt(ctx context.Context, attempt Attempt) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := ValidateAttempt(attempt); err != nil {
		return err
	}

	return s.repo.Add(ctx, attempt)
}

func (s *Service) Progress(ctx context.Context, learnerID, courseID string) (Progress, error) {
	if err := ctx.Err(); err != nil {
		return Progress{}, err
	}

	if strings.TrimSpace(learnerID) == "" {
		return Progress{}, ErrInvalidInput
	}

	if courseID != CourseID {
		return Progress{}, ErrUnknownCourse
	}
	// TODO(candidate 2): Use FixedCourse and s.repo to calculate the documented progress rules.
	return Progress{}, ErrNotImplemented
}
