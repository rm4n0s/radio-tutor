package submission

import (
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
)

// Event is something that happened to a Session. The concrete types are
// PageOpened, UploadStarted, PhotosReceived, Grading, Graded, Failed,
// Cancelled, Expired and FirewallHint; a consumer switches on the type.
//
// Events are sent on Session.Events, a buffered channel. The service never
// blocks on a slow consumer: when the buffer is full the oldest event is
// dropped to make room for the new one. Events are produced in the order they
// happen, a terminal event (Graded, Cancelled, Expired) is always the last one
// and is never lost, and the channel is closed right after it. Failed is not
// terminal: Retry can follow it.
type Event interface {
	event()
}

// PageOpened says that a phone opened the page of the token. It is sent for
// the first opening only.
type PageOpened struct{}

// UploadStarted says that a phone started to send photos.
type UploadStarted struct{}

// PhotosReceived says that the photos arrived and were stored.
type PhotosReceived struct {
	// Count is the number of photos.
	Count int
}

// Grading says that grading started (after the photos arrived, and again after
// a Retry).
type Grading struct{}

// Graded is the end of a successful submission: the results are applied to the
// save file.
type Graded struct {
	// Results are the grading results as stored in the submission, with
	// CountedAttempt set.
	Results []progress.Result
	// Changes says what applying them did: the exercises that were passed,
	// the hints that were unlocked, the exercises that need a review.
	Changes progress.Changes
	// PageNotes are the grader's remarks on the page as a whole.
	PageNotes string
	// Provider is the name of the provider that graded.
	Provider string
	// Model is the model that answered.
	Model string
	// Downgraded is not empty when the provider had to use a weaker
	// structured output mode than the configured one (SPEC §11.2): the UI
	// tells the parent to change the setting to it.
	Downgraded grading.StructuredOutput
}

// Failed says that the submission could not be graded (or that its photos
// could not be stored). The submission is "failed" in the save file.
type Failed struct {
	// Err is the cause, an *errors.Error. NoProvider(Err) is true when no
	// provider is active, which the UI shows as "Ask a parent to set up the
	// checker".
	Err error
	// CanRetry is true when Service.Retry can grade the stored photos again,
	// false when there are no photos (the child must start a new submission).
	CanRetry bool
}

// Cancelled is the end of a submission that was cancelled: by Cancel, by a
// newer Open for the same student, or by Service.Close.
type Cancelled struct{}

// Expired is the end of a submission whose photos did not arrive within 30
// minutes. The token no longer works.
type Expired struct{}

// FirewallHint says that no phone has opened the page 60 seconds after Open
// (SPEC §10.2): the dialog shows the hint about the firewall. It is sent once,
// and not at all when the page was opened in time.
type FirewallHint struct{}

func (PageOpened) event()     {}
func (UploadStarted) event()  {}
func (PhotosReceived) event() {}
func (Grading) event()        {}
func (Graded) event()         {}
func (Failed) event()         {}
func (Cancelled) event()      {}
func (Expired) event()        {}
func (FirewallHint) event()   {}
