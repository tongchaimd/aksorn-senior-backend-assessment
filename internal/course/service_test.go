package course_test

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"testing"

	"github.com/aksornproject/aksorn-senior-backend-assessment/internal/course"
	"github.com/aksornproject/aksorn-senior-backend-assessment/internal/infra"
)

func TestRecordAttemptRejectsUnknownLesson(t *testing.T) {
	// Invalid input is rejected before storage is accessed.
	service := course.NewService(nil)

	attempt := course.Attempt{
		AttemptID: "example-001",
		LearnerID: "learner-001",
		CourseID:  course.CourseID,
		LessonID:  "missing-lesson",
		Score:     75,
	}
	if err := service.RecordAttempt(
		context.Background(),
		attempt,
	); !errors.Is(
		err,
		course.ErrUnknownLesson,
	) {
		t.Fatalf("RecordAttempt error = %v, want ErrUnknownLesson", err)
	}
}

func TestRecordAttemptStoresAttempt(t *testing.T) {
	service, repo := newPostgresService(t)
	attempt := newAttempt(uniqueID("learner"), "introduction", 60)

	if err := service.RecordAttempt(t.Context(), attempt); err != nil {
		t.Fatalf("RecordAttempt error = %v", err)
	}

	assertStored(t, repo, attempt.LearnerID, []course.Attempt{attempt})
}

func TestRecordAttemptStoresMultipleAttemptsForSameLesson(t *testing.T) {
	service, repo := newPostgresService(t)
	learnerID := uniqueID("learner")
	// Lower and boundary scores are still accepted attempts; nothing is overwritten.
	attempts := []course.Attempt{
		newAttempt(learnerID, "practice", 100),
		newAttempt(learnerID, "practice", 40),
		newAttempt(learnerID, "practice", 0),
	}
	recordAll(t, service, attempts)

	assertStored(t, repo, learnerID, attempts)
}

func TestRecordAttemptDoesNotStoreRejectedAttempts(t *testing.T) {
	service, repo := newPostgresService(t)
	learnerID := uniqueID("learner")

	tests := []struct {
		name   string
		modify func(*course.Attempt)
		want   error
	}{
		{"blank attempt id", func(a *course.Attempt) { a.AttemptID = " " }, course.ErrInvalidInput},
		{"score below range", func(a *course.Attempt) { a.Score = -1 }, course.ErrInvalidInput},
		{"score above range", func(a *course.Attempt) { a.Score = 101 }, course.ErrInvalidInput},
		{"unknown course", func(a *course.Attempt) { a.CourseID = "course-002" }, course.ErrUnknownCourse},
		{"course id is case-sensitive", func(a *course.Attempt) { a.CourseID = "Course-001" }, course.ErrUnknownCourse},
		{"unknown lesson", func(a *course.Attempt) { a.LessonID = "outro" }, course.ErrUnknownLesson},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempt := newAttempt(learnerID, "introduction", 60)
			tt.modify(&attempt)

			if err := service.RecordAttempt(t.Context(), attempt); !errors.Is(err, tt.want) {
				t.Fatalf("RecordAttempt error = %v, want %v", err, tt.want)
			}
		})
	}

	assertStored(t, repo, learnerID, nil)
}

func TestProgressFromRecordedAttempts(t *testing.T) {
	service, _ := newPostgresService(t)
	learnerID := uniqueID("learner")
	recordAll(t, service, []course.Attempt{
		newAttempt(learnerID, "introduction", 60),
		newAttempt(learnerID, "introduction", 30),
		newAttempt(learnerID, "practice", 69),
		newAttempt(learnerID, "challenge", 95),
	})

	got, err := service.Progress(t.Context(), learnerID, course.CourseID)
	if err != nil {
		t.Fatalf("Progress error = %v", err)
	}

	assertProgress(t, got, learnerID, progressWant{
		lessons: map[string]lessonWant{
			"introduction": scored(60, true),
			"practice":     scored(69, false),
			"challenge":    scored(95, true),
		},
		percent: 50,
	})
}

func TestProgressIsolatesLearners(t *testing.T) {
	service, _ := newPostgresService(t)
	finisher, beginner := uniqueID("learner"), uniqueID("learner")
	recordAll(t, service, []course.Attempt{
		newAttempt(finisher, "introduction", 100),
		newAttempt(beginner, "introduction", 20),
		newAttempt(finisher, "practice", 100),
	})

	got, err := service.Progress(t.Context(), beginner, course.CourseID)
	if err != nil {
		t.Fatalf("Progress error = %v", err)
	}

	assertProgress(t, got, beginner, progressWant{lessons: map[string]lessonWant{
		"introduction": scored(20, false), "practice": noAttempts, "challenge": noAttempts,
	}})

	got, err = service.Progress(t.Context(), finisher, course.CourseID)
	if err != nil {
		t.Fatalf("Progress error = %v", err)
	}

	assertProgress(t, got, finisher, progressWant{
		lessons: map[string]lessonWant{
			"introduction": scored(100, true), "practice": scored(100, true), "challenge": noAttempts,
		},
		percent:   100,
		completed: true,
	})
}

func TestProgressForLearnerWithoutAttempts(t *testing.T) {
	service, _ := newPostgresService(t)
	learnerID := uniqueID("learner")

	got, err := service.Progress(t.Context(), learnerID, course.CourseID)
	if err != nil {
		t.Fatalf("Progress error = %v, want empty progress", err)
	}

	assertProgress(t, got, learnerID, progressWant{lessons: map[string]lessonWant{
		"introduction": noAttempts, "practice": noAttempts, "challenge": noAttempts,
	}})
}

func TestProgressRejectsInvalidRequests(t *testing.T) {
	// Request validation happens before storage is accessed.
	service := course.NewService(nil)

	tests := []struct {
		name      string
		learnerID string
		courseID  string
		want      error
	}{
		{"blank learner", " ", course.CourseID, course.ErrInvalidInput},
		{"unknown course", "learner-001", "course-002", course.ErrUnknownCourse},
		{"course id is case-sensitive", "learner-001", "COURSE-001", course.ErrUnknownCourse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := service.Progress(t.Context(), tt.learnerID, tt.courseID); !errors.Is(
				err,
				tt.want,
			) {
				t.Fatalf("Progress error = %v, want %v", err, tt.want)
			}
		})
	}
}

func recordAll(t *testing.T, service *course.Service, attempts []course.Attempt) {
	t.Helper()

	for _, attempt := range attempts {
		if err := service.RecordAttempt(t.Context(), attempt); err != nil {
			t.Fatalf("RecordAttempt(%+v) error = %v", attempt, err)
		}
	}
}

// newPostgresService uses the supplied connection configuration, so the database from setup
// must be running. Tests use unique learner IDs because stored attempts persist between runs.
func newPostgresService(t *testing.T) (*course.Service, *course.Repository) {
	t.Helper()

	connections, err := infra.Open(t.Context())
	if err != nil {
		t.Fatalf("open connections (run the README setup first): %v", err)
	}

	t.Cleanup(connections.Close)

	repo := course.NewRepository(connections.Postgres)

	return course.NewService(repo), repo
}

func newAttempt(learnerID, lessonID string, score int) course.Attempt {
	return course.Attempt{
		AttemptID: uniqueID("attempt"),
		LearnerID: learnerID,
		CourseID:  course.CourseID,
		LessonID:  lessonID,
		Score:     score,
	}
}

func uniqueID(prefix string) string {
	return prefix + "-" + rand.Text()
}

func assertStored(t *testing.T, repo *course.Repository, learnerID string, want []course.Attempt) {
	t.Helper()

	got, err := repo.List(t.Context(), learnerID, course.CourseID)
	if err != nil {
		t.Fatalf("List error = %v", err)
	}

	if !slices.Equal(got, want) {
		t.Fatalf("stored attempts = %+v, want %+v", got, want)
	}
}
