package xiaohongshu

import (
	"errors"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

type fakeFeedDetailNavigatePage struct {
	navigateErr error
	waitErr     error
	navigatedTo []string
}

func (f *fakeFeedDetailNavigatePage) Navigate(url string) error {
	f.navigatedTo = append(f.navigatedTo, url)
	return f.navigateErr
}

func (f *fakeFeedDetailNavigatePage) WaitDOMStable(_ time.Duration, _ float64) error {
	return f.waitErr
}

type fakeScrollTopReader struct {
	values []int
	errors []error
	calls  int
}

func (f *fakeScrollTopReader) ReadScrollTop() (int, error) {
	idx := f.calls
	f.calls++
	if idx < len(f.errors) && f.errors[idx] != nil {
		return 0, f.errors[idx]
	}
	if idx < len(f.values) {
		return f.values[idx], nil
	}
	return 0, nil
}

func TestNavigateFeedDetailPage_WrapsNavigateError(t *testing.T) {
	page := &fakeFeedDetailNavigatePage{navigateErr: errors.New("boom")}

	err := navigateFeedDetailPage(page, "https://example.com/feed")
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "navigate feed detail failed: boom" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNavigateFeedDetailPage_WrapsDOMStableError(t *testing.T) {
	page := &fakeFeedDetailNavigatePage{waitErr: errors.New("not stable")}

	err := navigateFeedDetailPage(page, "https://example.com/feed")
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "wait feed detail DOM stable failed: not stable" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetScrollTopFromReader_RetriesAndReturnsValue(t *testing.T) {
	reader := &fakeScrollTopReader{
		errors: []error{errors.New("temporary")},
		values: []int{0, 128},
	}

	got := getScrollTopFromReader(reader)
	if got != 128 {
		t.Fatalf("expected scrollTop 128, got %d", got)
	}
	if reader.calls != 2 {
		t.Fatalf("expected 2 calls, got %d", reader.calls)
	}
}

func TestGetScrollTopFromReader_ReturnsZeroOnRepeatedErrors(t *testing.T) {
	reader := &fakeScrollTopReader{
		errors: []error{errors.New("a"), errors.New("b"), errors.New("c")},
	}

	got := getScrollTopFromReader(reader)
	if got != 0 {
		t.Fatalf("expected scrollTop 0, got %d", got)
	}
}

func TestCommentLoaderUpdateState_SkipsTotalCountLookupWhenNoTargetConfigured(t *testing.T) {
	calls := 0
	original := readTotalCommentCount
	readTotalCommentCount = func(_ *rod.Page) int {
		calls++
		return 42
	}
	defer func() { readTotalCommentCount = original }()

	loader := &commentLoader{
		config: CommentLoadConfig{MaxCommentItems: 0},
		state:  &loadState{},
	}

	loader.updateState(3)

	if calls != 0 {
		t.Fatalf("expected total comment lookup to be skipped, got %d call(s)", calls)
	}
}

func TestCommentLoaderUpdateState_SkipsTotalCountLookupEvenWhenTargetConfigured(t *testing.T) {
	calls := 0
	original := readTotalCommentCount
	readTotalCommentCount = func(_ *rod.Page) int {
		calls++
		return 42
	}
	defer func() { readTotalCommentCount = original }()

	loader := &commentLoader{
		config: CommentLoadConfig{MaxCommentItems: 30},
		state:  &loadState{},
	}

	loader.updateState(3)

	if calls != 0 {
		t.Fatalf("expected total comment lookup to be skipped, got %d call(s)", calls)
	}
}
