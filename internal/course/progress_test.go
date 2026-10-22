package course_test

import (
	"strconv"
	"testing"

	"github.com/aksornproject/aksorn-senior-backend-assessment/internal/course"
)

type lessonWant struct {
	best   *int
	passed bool
}

type progressWant struct {
	lessons   map[string]lessonWant
	percent   int
	completed bool
}

// noAttempts is the expected result for a lesson without any attempts.
var noAttempts = lessonWant{}

func scored(score int, passed bool) lessonWant {
	return lessonWant{best: &score, passed: passed}
}

func TestCalculateProgress(t *testing.T) {
	const learnerID = "learner-001"

	attempt := func(lessonID string, score int) course.Attempt {
		return newAttempt(learnerID, lessonID, score)
	}

	tests := []struct {
		name     string
		attempts []course.Attempt
		want     progressWant
	}{
		{
			name:     "no attempts returns every lesson with null best score",
			attempts: nil,
			want: progressWant{lessons: map[string]lessonWant{
				"introduction": noAttempts, "practice": noAttempts, "challenge": noAttempts,
			}},
		},
		{
			name: "one below each passing score fails",
			attempts: []course.Attempt{
				attempt("introduction", 59), attempt("practice", 69), attempt("challenge", 79),
			},
			want: progressWant{lessons: map[string]lessonWant{
				"introduction": scored(59, false),
				"practice":     scored(69, false),
				"challenge":    scored(79, false),
			}},
		},
		{
			name: "exactly each passing score passes",
			attempts: []course.Attempt{
				attempt("introduction", 60), attempt("practice", 70), attempt("challenge", 80),
			},
			want: progressWant{
				lessons: map[string]lessonWant{
					"introduction": scored(60, true),
					"practice":     scored(70, true),
					"challenge":    scored(80, true),
				},
				percent:   100,
				completed: true,
			},
		},
		{
			name:     "score of zero is reported as zero, not null",
			attempts: []course.Attempt{attempt("introduction", 0)},
			want: progressWant{lessons: map[string]lessonWant{
				"introduction": scored(0, false), "practice": noAttempts, "challenge": noAttempts,
			}},
		},
		{
			name: "best score is the highest attempt regardless of order",
			attempts: []course.Attempt{
				attempt("practice", 50), attempt("practice", 90), attempt("practice", 70),
			},
			want: progressWant{
				lessons: map[string]lessonWant{
					"introduction": noAttempts, "practice": scored(90, true), "challenge": noAttempts,
				},
				percent: 50,
			},
		},
		{
			name: "later lower score keeps the earlier pass and best score",
			attempts: []course.Attempt{
				attempt("introduction", 60), attempt("introduction", 10),
			},
			want: progressWant{
				lessons: map[string]lessonWant{
					"introduction": scored(60, true), "practice": noAttempts, "challenge": noAttempts,
				},
				percent: 50,
			},
		},
		{
			name: "later passing score turns a failed lesson into a pass",
			attempts: []course.Attempt{
				attempt("practice", 69), attempt("practice", 70),
			},
			want: progressWant{
				lessons: map[string]lessonWant{
					"introduction": noAttempts, "practice": scored(70, true), "challenge": noAttempts,
				},
				percent: 50,
			},
		},
		{
			name:     "optional lesson pass alone gives no progress",
			attempts: []course.Attempt{attempt("challenge", 100)},
			want: progressWant{lessons: map[string]lessonWant{
				"introduction": noAttempts, "practice": noAttempts, "challenge": scored(100, true),
			}},
		},
		{
			name: "one required pass with the optional pass is still half and incomplete",
			attempts: []course.Attempt{
				attempt("introduction", 100), attempt("practice", 69), attempt("challenge", 100),
			},
			want: progressWant{
				lessons: map[string]lessonWant{
					"introduction": scored(100, true),
					"practice":     scored(69, false),
					"challenge":    scored(100, true),
				},
				percent: 50,
			},
		},
		{
			name: "both required passes complete the course despite failing the optional lesson",
			attempts: []course.Attempt{
				attempt("introduction", 60), attempt("practice", 70), attempt("challenge", 79),
			},
			want: progressWant{
				lessons: map[string]lessonWant{
					"introduction": scored(60, true),
					"practice":     scored(70, true),
					"challenge":    scored(79, false),
				},
				percent:   100,
				completed: true,
			},
		},
		{
			name: "both required passes complete the course without an optional attempt",
			attempts: []course.Attempt{
				attempt("practice", 100), attempt("introduction", 100),
			},
			want: progressWant{
				lessons: map[string]lessonWant{
					"introduction": scored(100, true),
					"practice":     scored(100, true),
					"challenge":    noAttempts,
				},
				percent:   100,
				completed: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := course.CalculateProgress(course.FixedCourse(), learnerID, tt.attempts)
			assertProgress(t, got, learnerID, tt.want)
		})
	}
}

func assertProgress(t *testing.T, got course.Progress, learnerID string, want progressWant) {
	t.Helper()

	if got.LearnerID != learnerID || got.CourseID != course.CourseID {
		t.Errorf(
			"ids = (%q, %q), want (%q, %q)",
			got.LearnerID, got.CourseID, learnerID, course.CourseID,
		)
	}

	if got.ProgressPercent != want.percent || got.Completed != want.completed {
		t.Errorf(
			"progress = (%d%%, completed %t), want (%d%%, completed %t)",
			got.ProgressPercent, got.Completed, want.percent, want.completed,
		)
	}

	// Lesson order is unspecified, so compare by ID and require each lesson exactly once.
	if len(got.Lessons) != len(want.lessons) {
		t.Fatalf("got %d lessons, want %d: %+v", len(got.Lessons), len(want.lessons), got.Lessons)
	}

	seen := make(map[string]bool)

	for _, lesson := range got.Lessons {
		expected, ok := want.lessons[lesson.LessonID]
		if !ok || seen[lesson.LessonID] {
			t.Errorf("unexpected or duplicate lesson %q", lesson.LessonID)
			continue
		}

		seen[lesson.LessonID] = true

		if formatScore(lesson.BestScore) != formatScore(expected.best) ||
			lesson.Passed != expected.passed {
			t.Errorf(
				"%s = (best %s, passed %t), want (best %s, passed %t)",
				lesson.LessonID, formatScore(lesson.BestScore), lesson.Passed,
				formatScore(expected.best), expected.passed,
			)
		}
	}
}

func formatScore(score *int) string {
	if score == nil {
		return "null"
	}

	return strconv.Itoa(*score)
}
