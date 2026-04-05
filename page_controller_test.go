package main

import (
	"errors"
	"testing"

	"github.com/go-rod/rod"
)

func TestCDPPageControllerReusesWorkPage(t *testing.T) {
	connectCalls := 0
	createCalls := 0

	controller := newCDPPageControllerWithDeps(cdpPageControllerDeps{
		connect: func() (*rod.Browser, func() error, error) {
			connectCalls++
			return &rod.Browser{}, func() error { return nil }, nil
		},
		createPage: func(*rod.Browser) (*rod.Page, error) {
			createCalls++
			return &rod.Page{}, nil
		},
		closePage: func(*rod.Page) error { return nil },
	})

	lease1, err := controller.Acquire(pageRoleWork)
	if err != nil {
		t.Fatalf("acquire first lease: %v", err)
	}
	page1 := lease1.Page
	lease1.Release(nil)

	lease2, err := controller.Acquire(pageRoleWork)
	if err != nil {
		t.Fatalf("acquire second lease: %v", err)
	}
	page2 := lease2.Page
	lease2.Release(nil)

	if page1 != page2 {
		t.Fatalf("expected work page to be reused")
	}
	if connectCalls != 1 {
		t.Fatalf("expected one browser connection, got %d", connectCalls)
	}
	if createCalls != 1 {
		t.Fatalf("expected one page creation, got %d", createCalls)
	}
}

func TestCDPPageControllerResetsSlotAfterOperationError(t *testing.T) {
	createCalls := 0
	closeCalls := 0

	controller := newCDPPageControllerWithDeps(cdpPageControllerDeps{
		connect: func() (*rod.Browser, func() error, error) {
			return &rod.Browser{}, func() error { return nil }, nil
		},
		createPage: func(*rod.Browser) (*rod.Page, error) {
			createCalls++
			return &rod.Page{}, nil
		},
		closePage: func(*rod.Page) error {
			closeCalls++
			return nil
		},
	})

	lease1, err := controller.Acquire(pageRoleWork)
	if err != nil {
		t.Fatalf("acquire first lease: %v", err)
	}
	page1 := lease1.Page
	lease1.Release(errors.New("boom"))

	lease2, err := controller.Acquire(pageRoleWork)
	if err != nil {
		t.Fatalf("acquire second lease: %v", err)
	}
	page2 := lease2.Page
	lease2.Release(nil)

	if page1 == page2 {
		t.Fatalf("expected page to be recreated after error")
	}
	if createCalls != 2 {
		t.Fatalf("expected two page creations, got %d", createCalls)
	}
	if closeCalls != 1 {
		t.Fatalf("expected one page close, got %d", closeCalls)
	}
}

func TestCDPPageControllerSeparatesLoginAndWorkPages(t *testing.T) {
	createCalls := 0

	controller := newCDPPageControllerWithDeps(cdpPageControllerDeps{
		connect: func() (*rod.Browser, func() error, error) {
			return &rod.Browser{}, func() error { return nil }, nil
		},
		createPage: func(*rod.Browser) (*rod.Page, error) {
			createCalls++
			return &rod.Page{}, nil
		},
		closePage: func(*rod.Page) error { return nil },
	})

	workLease, err := controller.Acquire(pageRoleWork)
	if err != nil {
		t.Fatalf("acquire work lease: %v", err)
	}
	loginLease, err := controller.Acquire(pageRoleLogin)
	if err != nil {
		t.Fatalf("acquire login lease: %v", err)
	}

	if workLease.Page == loginLease.Page {
		t.Fatalf("expected login and work to use distinct pages")
	}
	if createCalls != 2 {
		t.Fatalf("expected two page creations, got %d", createCalls)
	}

	loginLease.Release(nil)
	workLease.Release(nil)
}
