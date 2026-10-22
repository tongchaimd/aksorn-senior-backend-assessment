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

	attempts, err := s.repo.List(ctx, learnerID, courseID)
	if err != nil {
		return Progress{}, err
	}

	return calculateProgress(FixedCourse(), learnerID, attempts), nil
}

// calculateProgress applies the progress rules to one learner's attempts for the course.
// Pass status comes from the best score, so a later lower score cannot remove a pass.
func calculateProgress(course Course, learnerID string, attempts []Attempt) Progress {
	best := make(map[string]int)
	for _, attempt := range attempts {
		// Check presence, not the zero value, so a real score of 0 is kept.
		if current, ok := best[attempt.LessonID]; !ok || attempt.Score > current {
			best[attempt.LessonID] = attempt.Score
		}
	}

	lessons := make([]LessonProgress, 0, len(course.Lessons))
	required, passedRequired := 0, 0

	for _, lesson := range course.Lessons {
		result := LessonProgress{LessonID: lesson.ID}
		if score, ok := best[lesson.ID]; ok {
			result.BestScore = &score
			result.Passed = score >= lesson.PassingScore
		}

		if lesson.Required {
			required++

			if result.Passed {
				passedRequired++
			}
		}

		lessons = append(lessons, result)
	}

	percent := 0
	if required > 0 {
		percent = passedRequired * 100 / required
	}

	return Progress{
		LearnerID:       learnerID,
		CourseID:        course.ID,
		Lessons:         lessons,
		ProgressPercent: percent,
		Completed:       required > 0 && passedRequired == required,
	}
}
